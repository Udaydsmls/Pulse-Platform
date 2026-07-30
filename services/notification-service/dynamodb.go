package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Store writes notification logs to DynamoDB, partitioned by user so a
// customer's history can be read back in one query.
type Store struct {
	client *dynamodb.Client
	table  string
}

func (s *Store) Save(ctx context.Context, entry *Log) error {
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			"UserID":    &types.AttributeValueMemberS{Value: entry.UserID},
			"Timestamp": &types.AttributeValueMemberS{Value: entry.Timestamp},
			"EventType": &types.AttributeValueMemberS{Value: entry.EventType},
			"Channel":   &types.AttributeValueMemberS{Value: entry.Channel},
			"Message":   &types.AttributeValueMemberS{Value: entry.Message},
		},
	})
	return err
}
