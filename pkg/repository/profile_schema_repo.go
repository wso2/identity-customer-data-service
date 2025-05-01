package repositories

import (
	"context"
	"fmt"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
	"time"
)

type ProfileSchemaRepository struct {
	Collection *mongo.Collection
}

func NewProfileSchemaRepository(db *mongo.Database, collection string) *ProfileSchemaRepository {
	return &ProfileSchemaRepository{
		Collection: db.Collection(collection),
	}
}

func (repo *ProfileSchemaRepository) UpsertEnrichmentRule(rule models.ProfileEnrichmentRule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"rule_id": rule.RuleId} // assuming rule_id is unique
	update := bson.M{"$set": rule}

	opts := options.Update().SetUpsert(true)

	_, err := repo.Collection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (repo *ProfileSchemaRepository) GetProfileEnrichmentRules() ([]models.ProfileEnrichmentRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := repo.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.ProfileEnrichmentRule
	err = cursor.All(ctx, &rules)
	return rules, err
}

func (repo *ProfileSchemaRepository) GetEnrichmentRulesByFilter(filters []string) ([]models.ProfileEnrichmentRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mongoFilter := bson.M{}

	for _, f := range filters {
		tokens := strings.SplitN(f, " ", 3)
		if len(tokens) != 3 {
			return nil, fmt.Errorf("invalid filter format: %s", f)
		}

		field := tokens[0]
		operator := strings.ToLower(tokens[1])
		value := strings.TrimSpace(tokens[2])

		switch operator {
		case "sw": // starts with
			mongoFilter[field] = bson.M{"$regex": fmt.Sprintf("^%s", value)}
		case "co": // contains
			mongoFilter[field] = bson.M{"$regex": value}
		case "eq":
			mongoFilter[field] = value
		default:
			return nil, fmt.Errorf("unsupported operator: %s", operator)
		}
	}

	cursor, err := repo.Collection.Find(ctx, mongoFilter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.ProfileEnrichmentRule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

func (repo *ProfileSchemaRepository) GetSchemaRule(traitId string) (models.ProfileEnrichmentRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"rule_id": traitId}

	var rule models.ProfileEnrichmentRule
	err := repo.Collection.FindOne(ctx, filter).Decode(&rule)
	if err != nil {
		return models.ProfileEnrichmentRule{}, err
	}

	return rule, nil
}

func (repo *ProfileSchemaRepository) DeleteSchemaRule(attribute string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := repo.Collection.DeleteOne(ctx, bson.M{"rule_id": attribute})
	return err
}
