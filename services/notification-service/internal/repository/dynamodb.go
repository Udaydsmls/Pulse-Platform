package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/pulse-platform/notification-service/internal/domain"
)

// NotificationRepository handles persistence for notification logs in DynamoDB.
type NotificationRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewNotificationRepository creates a new NotificationRepository targeting the given DynamoDB table.
func NewNotificationRepository(client *dynamodb.Client, tableName string) *NotificationRepository {
	return &NotificationRepository{client: client, tableName: tableName}
}

// Save persists a NotificationLog to DynamoDB using UserID as partition key and Timestamp as sort key.
func (r *NotificationRepository) Save(ctx context.Context, log *domain.NotificationLog) error {
	_, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item: map[string]types.AttributeValue{
			"UserID":    &types.AttributeValueMemberS{Value: log.UserID},
			"Timestamp": &types.AttributeValueMemberS{Value: log.Timestamp},
			"EventType": &types.AttributeValueMemberS{Value: log.EventType},
			"Channel":   &types.AttributeValueMemberS{Value: log.Channel},
			"Status":    &types.AttributeValueMemberS{Value: log.Status},
			"Payload":   &types.AttributeValueMemberS{Value: log.Payload},
			"CreatedAt": &types.AttributeValueMemberS{Value: log.CreatedAt.String()},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb put notification log: %w", err)
	}
	return nil
}

// ListByUser returns all notification logs for the given user ID.
func (r *NotificationRepository) ListByUser(ctx context.Context, userID string) ([]*domain.NotificationLog, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("UserID = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb query notifications: %w", err)
	}

	logs := make([]*domain.NotificationLog, 0, len(out.Items))
	for _, item := range out.Items {
		log := &domain.NotificationLog{}
		if v, ok := item["UserID"].(*types.AttributeValueMemberS); ok {
			log.UserID = v.Value
		}
		if v, ok := item["Timestamp"].(*types.AttributeValueMemberS); ok {
			log.Timestamp = v.Value
		}
		if v, ok := item["EventType"].(*types.AttributeValueMemberS); ok {
			log.EventType = v.Value
		}
		if v, ok := item["Channel"].(*types.AttributeValueMemberS); ok {
			log.Channel = v.Value
		}
		if v, ok := item["Status"].(*types.AttributeValueMemberS); ok {
			log.Status = v.Value
		}
		if v, ok := item["Payload"].(*types.AttributeValueMemberS); ok {
			log.Payload = v.Value
		}
		logs = append(logs, log)
	}
	return logs, nil
}
