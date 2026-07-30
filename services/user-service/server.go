package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulse-platform/user-service/gen/pb"
)

// UserServer implements the UserService gRPC API.
type UserServer struct {
	pb.UnimplementedUserServiceServer
	db       *DB
	producer *Producer
	secret   string
}

// Register creates an account and returns a JWT.
func (s *UserServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.GetEmail() == "" || req.GetPassword() == "" || req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "email, password and name are required")
	}

	hash, err := hashPassword(req.GetPassword())
	if err != nil {
		log.Printf("hash password: %v", err)
		return nil, status.Error(codes.Internal, "could not create account")
	}

	user := &User{
		ID:           uuid.NewString(),
		Email:        req.GetEmail(),
		Name:         req.GetName(),
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := user.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.db.Insert(ctx, user); err != nil {
		// The email column is unique, so a duplicate signup lands here.
		return nil, status.Error(codes.AlreadyExists, "email already registered")
	}

	// The account exists either way, so a publish failure is logged rather than
	// failing the signup — it only costs the customer a welcome email.
	if err := s.producer.PublishUserCreated(ctx, user.ID, user.Email); err != nil {
		log.Printf("publish user.created for %s: %v", user.ID, err)
	}

	return &pb.RegisterResponse{
		UserId: user.ID,
		Token:  s.issueToken(user),
	}, nil
}

// Login checks a password and returns a JWT.
func (s *UserServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.GetEmail() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	user, err := s.db.FindByEmail(ctx, req.GetEmail())
	// The same message for "no such user" and "wrong password" keeps the API
	// from confirming which emails are registered.
	if err != nil || !checkPassword(user.PasswordHash, req.GetPassword()) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return &pb.LoginResponse{
		Token:  s.issueToken(user),
		UserId: user.ID,
	}, nil
}

// GetUser returns a user profile.
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := s.db.FindByID(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return &pb.GetUserResponse{
		UserId:    user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}, nil
}

// issueToken signs a JWT. A signing failure means the service is misconfigured,
// so it is logged rather than surfaced to the caller.
func (s *UserServer) issueToken(user *User) string {
	token, err := newToken(s.secret, user.ID, user.Email)
	if err != nil {
		log.Printf("sign token for %s: %v", user.ID, err)
	}
	return token
}
