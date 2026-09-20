package pool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// testTask — тестовый тип, похожий на worker.DeleteTask.
// resetCnt считает вызовы Reset для проверки.
type testTask struct {
	UserID   string
	ShortURL string
	Data     []string
	resetCnt int
}

// Reset сбрасывает поля testTask к нулевым значениям.
// Обрабатывает nil-получателя: вызов на nil не паникует.
func (t *testTask) Reset() {
	if t == nil {
		return
	}
	t.UserID = ""
	t.ShortURL = ""
	t.Data = t.Data[:0]
	t.resetCnt++
}

// newTask — фабрика объектов testTask для пула.
func newTask() *testTask {
	return &testTask{}
}

// TestPool_GetPutReuse проверяет базовое поведение: объект,
// возвращённый в пул, переиспользуется при следующем Get.
func TestPool_GetPutReuse(t *testing.T) {
	p := New(newTask)

	obj := p.Get()
	assert.NotNil(t, obj, "Get должен вернуть не-nil объект из newFn")

	p.Put(obj)

	// sync.Pool в пределах одной горутины без GC хранит последний
	// положенный объект в приватном слоте — Get возвращает его же.
	reused := p.Get()
	assert.Same(t, obj, reused, "объект должен быть переиспользован")
}

// TestPool_PutCallsReset проверяет, что Put сбрасывает объект
// и что Get возвращает уже сброшенный объект.
func TestPool_PutCallsReset(t *testing.T) {
	p := New(newTask)

	obj := p.Get()
	obj.UserID = "123"
	obj.ShortURL = "abc"
	obj.Data = append(obj.Data, "x", "y")

	p.Put(obj)

	assert.Empty(t, obj.UserID, "Put должен сбросить UserID")
	assert.Empty(t, obj.ShortURL, "Put должен сбросить ShortURL")
	assert.Empty(t, obj.Data, "Put должен обрезать Data")
	assert.Equal(t, 1, obj.resetCnt, "Reset должен быть вызван один раз")

	reused := p.Get()
	assert.Same(t, obj, reused)
	assert.Empty(t, reused.UserID, "объект из пула должен быть сброшен")
	assert.Empty(t, reused.ShortURL, "объект из пула должен быть сброшен")
}

// TestPool_NewNil проверяет пул без фабрики: Get возвращает
// zero value типа T и не паникует.
func TestPool_NewNil(t *testing.T) {
	p := New[*testTask](nil)

	obj := p.Get()
	assert.Nil(t, obj, "без newFn Get должен возвращать zero value (nil)")

	// Повторный Get — тоже zero value, без паники
	obj = p.Get()
	assert.Nil(t, obj)
}

// TestPool_NewFnCalledOnEmpty проверяет, что фабрика вызывается,
// когда пул пуст.
func TestPool_NewFnCalledOnEmpty(t *testing.T) {
	created := 0
	p := New(func() *testTask {
		created++
		return newTask()
	})

	// Пул пуст: каждый Get создаёт новый объект
	a := p.Get()
	b := p.Get()
	assert.NotSame(t, a, b, "пустой пул должен создавать новые объекты")
	assert.Equal(t, 2, created, "newFn должен быть вызван дважды")

	// После Put объект переиспользуется, newFn не вызывается
	p.Put(a)
	got := p.Get()
	assert.Same(t, a, got, "объект из пула должен быть переиспользован")
	assert.Equal(t, 2, created, "newFn не должен вызываться при наличии свободных объектов")
}

// TestPool_DrainAndRefill проверяет сценарий: забрали все объекты
// из пула, потом вернули обратно и снова забрали.
func TestPool_DrainAndRefill(t *testing.T) {
	p := New(newTask)

	// Набираем объекты, пока пул не опустеет — фабрика создаёт новые
	const n = 5
	objs := make([]*testTask, 0, n)
	seen := make(map[*testTask]struct{})
	for len(objs) < n {
		obj := p.Get()
		if _, dup := seen[obj]; dup {
			t.Fatal("пустой пул не должен возвращать один объект дважды")
		}
		seen[obj] = struct{}{}
		objs = append(objs, obj)
	}

	// Возвращаем все объекты обратно
	for _, obj := range objs {
		obj.UserID = "u"
		p.Put(obj)
	}

	// Снова забираем: пул отдаёт ранее возвращённые объекты
	refilled := make([]*testTask, 0, n)
	for i := 0; i < n; i++ {
		obj := p.Get()
		assert.Empty(t, obj.UserID, "переиспользованный объект должен быть сброшен")
		refilled = append(refilled, obj)
	}

	// Все возвращённые объекты должны быть из исходного набора
	for _, obj := range refilled {
		if _, ok := seen[obj]; !ok {
			t.Fatal("пул должен отдавать ранее возвращённые объекты")
		}
	}
}

// TestPool_Concurrent проверяет потокобезопасность: горутины
// одновременно выполняют Get и Put. Тест рассчитан на запуск
// с флагом -race.
func TestPool_Concurrent(t *testing.T) {
	p := New(newTask)

	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				obj := p.Get()
				obj.UserID = "u"
				obj.ShortURL = "s"
				obj.Data = append(obj.Data, "d")
				p.Put(obj)
			}
		}()
	}
	wg.Wait()

	// Пул по-прежнему работоспособен
	obj := p.Get()
	assert.NotNil(t, obj)
	assert.Empty(t, obj.UserID, "объект из пула должен быть сброшен")
	p.Put(obj)
}

// TestPool_PutNilSafeObject проверяет Put объекта, чей Reset
// корректно обрабатывает nil (zero value при newFn == nil).
func TestPool_PutNilSafeObject(t *testing.T) {
	p := New[*testTask](nil)

	// Get возвращает nil; Reset на nil не паникует,
	// sync.Pool игнорирует nil-значения
	obj := p.Get()
	assert.Nil(t, obj)
	p.Put(obj) // не должно паниковать
}
