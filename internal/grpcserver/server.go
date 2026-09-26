// Package grpcserver реализует gRPC-сервер сервиса сокращения URL,
// работающий параллельно с HTTP на отдельном порту.
package grpcserver

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/b602op/shortener/gen/pb/shortener/v1"
	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/domain"
	"github.com/b602op/shortener/internal/service"
)

// Server реализует pb.ShortenerServiceServer поверх общего слоя
// бизнес-логики ShortenerService.
type Server struct {
	pb.UnimplementedShortenerServiceServer

	svc  *service.ShortenerService
	auth *auth.Service
}

// NewServer создаёт gRPC-сервер. auth используется для проверки JWT
// из metadata — тот же секрет, что и у HTTP-middleware.
func NewServer(svc *service.ShortenerService, auth *auth.Service) *Server {
	return &Server{svc: svc, auth: auth}
}

// ShortenURL сокращает URL (аналог POST /api/shorten).
//
// Статусы: Unauthenticated — нет/невалидный токен, InvalidArgument —
// невалидный URL, Internal — ошибка хранилища. Дубликат не ошибка:
// возвращается существующая короткая ссылка.
func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	userID, err := s.userIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.svc.ShortenURL(ctx, userID, req.GetUrl())
	if err != nil {
		if errors.Is(err, domain.ErrInvalidURL) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		// Дубликат — идемпотентный успех: возвращаем существующую ссылку.
		if errors.Is(err, domain.ErrDuplicateURL) {
			return pb.URLShortenResponse_builder{Result: &result}.Build(), nil
		}
		slog.Error("gRPC ShortenURL: ошибка сохранения", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return pb.URLShortenResponse_builder{Result: &result}.Build(), nil
}

// ExpandURL возвращает оригинальный URL по короткому идентификатору
// (аналог GET /{id}, но без HTTP-редиректа).
//
// Статусы: NotFound — ссылка не найдена или удалена, Internal — ошибка хранилища.
func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	result, err := s.svc.ExpandURL(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "not found")
		}
		if errors.Is(err, domain.ErrDeleted) {
			return nil, status.Error(codes.NotFound, "deleted")
		}
		slog.Error("gRPC ExpandURL: ошибка выбора", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return pb.URLExpandResponse_builder{Result: &result}.Build(), nil
}

// ListUserURLs возвращает все URL, сокращённые пользователем
// (аналог GET /api/user/urls).
//
// Статусы: Unauthenticated — нет/невалидный токен, Internal — ошибка хранилища.
func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, err := s.userIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}

	records, err := s.svc.ListUserURLs(ctx, userID)
	if err != nil {
		slog.Error("gRPC ListUserURLs: ошибка выбора", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	urls := make([]*pb.URLData, 0, len(records))
	for _, record := range records {
		shortURL := s.svc.FullShortURL(record.ShortURL)
		originalURL := record.OriginalURL
		urls = append(urls, pb.URLData_builder{
			ShortUrl:    &shortURL,
			OriginalUrl: &originalURL,
		}.Build())
	}

	return pb.UserURLsResponse_builder{Url: urls}.Build(), nil
}

// userIDFromMetadata извлекает и проверяет идентификатор пользователя
// из metadata "authorization: Bearer <token>". JWT проверяется тем же
// секретом, что и кука HTTP-middleware.
func (s *Server) userIDFromMetadata(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization")
	}

	const prefix = "Bearer "
	authHeader := values[0]
	if !strings.HasPrefix(authHeader, prefix) {
		return "", status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	token := strings.TrimPrefix(authHeader, prefix)
	userID, err := s.auth.Verify(token)
	if err != nil {
		return "", status.Error(codes.Unauthenticated, "invalid token")
	}

	return userID, nil
}
