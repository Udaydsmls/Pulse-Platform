package server

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulse-platform/user-service/gen/pb"
	"github.com/pulse-platform/user-service/internal/auth"
	"github.com/pulse-platform/user-service/internal/config"
	"github.com/pulse-platform/user-service/internal/domain"
	"github.com/pulse-platform/user-service/internal/kafka"
	"github.com/pulse-platform/user-service/internal/repository"

	"github.com/google/uuid"
)

// bcryptHasher implements domain.PasswordHasher using bcrypt.
type bcryptHasher struct{}

func (bcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(b), nil
}

func (bcryptHasher) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// UserServer implements the gRPC UserServiceServer interface.
type UserServer struct {
	pb.UnimplementedUserServiceServer
	repo        *repository.UserRepository
	producer    *kafka.KafkaProducer
	jwt         *auth.JWTManager
	cfg         *config.Config
	logger      *zap.Logger
	googleOAuth auth.OAuthProvider
	githubOAuth auth.OAuthProvider
	hasher      domain.PasswordHasher
}

// NewUserServer constructs a UserServer with all required dependencies.
func NewUserServer(
	repo *repository.UserRepository,
	producer *kafka.KafkaProducer,
	jwt *auth.JWTManager,
	cfg *config.Config,
	logger *zap.Logger,
	googleOAuth auth.OAuthProvider,
	githubOAuth auth.OAuthProvider,
) *UserServer {
	return &UserServer{
		repo:        repo,
		producer:    producer,
		jwt:         jwt,
		cfg:         cfg,
		logger:      logger,
		googleOAuth: googleOAuth,
		githubOAuth: githubOAuth,
		hasher:      bcryptHasher{},
	}
}

// Register creates a new local user account and returns a JWT.
func (s *UserServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "email, password, and name are required")
	}

	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		s.logger.Error("hash password", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to process password")
	}

	user := domain.NewUser(req.Email, req.Name, hash)
	user.ID = uuid.New().String()

	if err := user.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("create user", zap.Error(err))
		return nil, status.Error(codes.AlreadyExists, "user already exists")
	}

	token, err := s.jwt.Generate(user.ID, user.Email)
	if err != nil {
		s.logger.Error("generate jwt", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	if err := s.producer.PublishUserCreated(ctx, user.ID, user.Email, "default"); err != nil {
		s.logger.Warn("publish user.created event", zap.Error(err))
	}

	return &pb.RegisterResponse{UserId: user.ID, Token: token}, nil
}

// Login authenticates a local user and returns a JWT.
func (s *UserServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Error(codes.NotFound, "invalid credentials")
	}

	if !user.MatchesPassword(req.Password, s.hasher) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	token, err := s.jwt.Generate(user.ID, user.Email)
	if err != nil {
		s.logger.Error("generate jwt", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &pb.LoginResponse{Token: token, UserId: user.ID}, nil
}

// GetUser retrieves a user's profile by ID.
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := s.repo.FindByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return &pb.GetUserResponse{
		UserId:    user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.String(),
		Provider:  string(user.Provider),
	}, nil
}

// OAuthLogin authenticates a user via an OAuth provider and returns a JWT.
func (s *UserServer) OAuthLogin(ctx context.Context, req *pb.OAuthLoginRequest) (*pb.OAuthLoginResponse, error) {
	if req.Code == "" || req.Provider == "" {
		return nil, status.Error(codes.InvalidArgument, "code and provider are required")
	}

	var provider auth.OAuthProvider
	var domainProvider domain.Provider

	switch req.Provider {
	case "google":
		provider = s.googleOAuth
		domainProvider = domain.ProviderGoogle
	case "github":
		provider = s.githubOAuth
		domainProvider = domain.ProviderGithub
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported provider")
	}

	info, err := provider.ExchangeCode(ctx, req.Code)
	if err != nil {
		s.logger.Error("oauth exchange", zap.String("provider", req.Provider), zap.Error(err))
		return nil, status.Error(codes.Unauthenticated, "oauth exchange failed")
	}

	user, err := s.repo.FindByProviderID(ctx, domainProvider, info.ID)
	if err != nil {
		user = domain.NewOAuthUser(info.Email, info.Name, domainProvider, info.ID)
		user.ID = uuid.New().String()

		if createErr := s.repo.Create(ctx, user); createErr != nil {
			existing, findErr := s.repo.FindByEmail(ctx, info.Email)
			if findErr != nil {
				s.logger.Error("create oauth user", zap.Error(createErr))
				return nil, status.Error(codes.Internal, "failed to create user")
			}
			user = existing
		} else {
			if pubErr := s.producer.PublishUserCreated(ctx, user.ID, user.Email, "default"); pubErr != nil {
				s.logger.Warn("publish user.created event", zap.Error(pubErr))
			}
		}
	}

	token, err := s.jwt.Generate(user.ID, user.Email)
	if err != nil {
		s.logger.Error("generate jwt", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &pb.OAuthLoginResponse{Token: token, UserId: user.ID}, nil
}
