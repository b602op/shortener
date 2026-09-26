package grpcserver

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/b602op/shortener/gen/pb/shortener/v1"
	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/repository"
	"github.com/b602op/shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBaseURL   = "http://localhost:8080"
	testSecretKey = "test-secret"
	testUserID    = "00000000-0000-0000-0000-000000000001"
)

// setupBufconn поднимает gRPC-сервер в памяти (bufconn) и возвращает клиента.
func setupBufconn(t *testing.T, store repository.Store) (pb.ShortenerServiceClient, *auth.Service) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)

	authService := auth.NewService(testSecretKey)
	svc := service.NewShortenerService(store, testBaseURL)

	srv := grpc.NewServer()
	pb.RegisterShortenerServiceServer(srv, NewServer(svc, authService))

	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewShortenerServiceClient(conn), authService
}

// authCtx возвращает контекст с metadata authorization: Bearer <token>.
func authCtx(t *testing.T, authService *auth.Service, userID string) context.Context {
	t.Helper()

	token, err := authService.BuildJWTStringWithUserID(userID)
	require.NoError(t, err)

	return metadata.AppendToOutgoingContext(
		context.Background(),
		"authorization", "Bearer "+token,
	)
}

func TestShortenURL_Success(t *testing.T) {
	client, authService := setupBufconn(t, repository.NewFileStorage())

	url := "https://example.com/grpc"
	resp, err := client.ShortenURL(authCtx(t, authService, testUserID),
		pb.URLShortenRequest_builder{Url: &url}.Build())
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetResult())
	assert.Contains(t, resp.GetResult(), testBaseURL+"/")
}

func TestShortenURL_Duplicate(t *testing.T) {
	client, authService := setupBufconn(t, repository.NewFileStorage())

	url := "https://example.com/duplicate"
	ctx := authCtx(t, authService, testUserID)

	first, err := client.ShortenURL(ctx, pb.URLShortenRequest_builder{Url: &url}.Build())
	require.NoError(t, err)

	// Дубликат — идемпотентный успех с той же короткой ссылкой
	second, err := client.ShortenURL(ctx, pb.URLShortenRequest_builder{Url: &url}.Build())
	require.NoError(t, err)
	assert.Equal(t, first.GetResult(), second.GetResult())
}

func TestShortenURL_InvalidURL(t *testing.T) {
	client, authService := setupBufconn(t, repository.NewFileStorage())

	tests := []struct {
		name string
		url  string
	}{
		{"пустой URL", ""},
		{"не http(s)", "ftp://example.com"},
		{"мусор", "not-a-url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.url
			_, err := client.ShortenURL(authCtx(t, authService, testUserID),
				pb.URLShortenRequest_builder{Url: &url}.Build())
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestShortenURL_Unauthenticated(t *testing.T) {
	client, _ := setupBufconn(t, repository.NewFileStorage())

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{"без metadata", context.Background()},
		{"без authorization", metadata.AppendToOutgoingContext(context.Background(), "x-other", "value")},
		{"неверный формат", metadata.AppendToOutgoingContext(context.Background(), "authorization", "token-without-bearer")},
		{"невалидный токен", metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer tampered.token")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "https://example.com"
			_, err := client.ShortenURL(tt.ctx, pb.URLShortenRequest_builder{Url: &url}.Build())
			require.Error(t, err)
			assert.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}

func TestExpandURL_Success(t *testing.T) {
	store := repository.NewFileStorage()
	require.NoError(t, store.Insert(testUserID, "https://example.com/original", "known1"))

	client, authService := setupBufconn(t, store)

	id := "known1"
	resp, err := client.ExpandURL(authCtx(t, authService, testUserID),
		pb.URLExpandRequest_builder{Id: &id}.Build())
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/original", resp.GetResult())
}

func TestExpandURL_NotFound(t *testing.T) {
	client, authService := setupBufconn(t, repository.NewFileStorage())

	id := "missing"
	_, err := client.ExpandURL(authCtx(t, authService, testUserID),
		pb.URLExpandRequest_builder{Id: &id}.Build())
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestExpandURL_Deleted(t *testing.T) {
	store := repository.NewFileStorage()
	require.NoError(t, store.Insert(testUserID, "https://example.com/deleted", "gone1"))
	require.NoError(t, store.DeleteByUser(testUserID, []string{"gone1"}))

	client, authService := setupBufconn(t, store)

	id := "gone1"
	_, err := client.ExpandURL(authCtx(t, authService, testUserID),
		pb.URLExpandRequest_builder{Id: &id}.Build())
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestListUserURLs_Success(t *testing.T) {
	store := repository.NewFileStorage()
	require.NoError(t, store.Insert(testUserID, "https://example.com/1", "id1"))
	require.NoError(t, store.Insert(testUserID, "https://example.com/2", "id2"))

	client, authService := setupBufconn(t, store)

	resp, err := client.ListUserURLs(authCtx(t, authService, testUserID), &emptypb.Empty{})
	require.NoError(t, err)

	urls := resp.GetUrl()
	require.Len(t, urls, 2)
	for _, u := range urls {
		assert.Contains(t, u.GetShortUrl(), testBaseURL+"/")
		assert.Contains(t, u.GetOriginalUrl(), "https://example.com/")
	}
}

func TestListUserURLs_Empty(t *testing.T) {
	client, authService := setupBufconn(t, repository.NewFileStorage())

	resp, err := client.ListUserURLs(authCtx(t, authService, testUserID), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Empty(t, resp.GetUrl())
}

func TestListUserURLs_Unauthenticated(t *testing.T) {
	client, _ := setupBufconn(t, repository.NewFileStorage())

	_, err := client.ListUserURLs(context.Background(), &emptypb.Empty{})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
