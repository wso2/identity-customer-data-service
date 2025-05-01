package repositories

import (
	"context"
	"github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/logger"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

// UnificationRuleRepository handles DB operations for unification rules
type UnificationRuleRepository struct {
	Collection *mongo.Collection
}

// NewUnificationRuleRepository initializes a repository
func NewUnificationRuleRepository(db *mongo.Database, collectionName string) *UnificationRuleRepository {
	return &UnificationRuleRepository{
		Collection: db.Collection(collectionName),
	}
}

// AddUnificationRule Inserts a new unification rule
func (repo *UnificationRuleRepository) AddUnificationRule(rule models.UnificationRule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := repo.Collection.InsertOne(ctx, rule)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileCreatingUnificationRules, err)
	}

	logger.Info("Unification rule created successfully: " + rule.RuleName)
	return nil
}

// GetUnificationRules  Retrieves all unification rules
func (repo *UnificationRuleRepository) GetUnificationRules() ([]models.UnificationRule, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cursor, err := repo.Collection.Find(ctx, bson.M{})
	if err != nil {
		logger.Info("Error occurred while fetching resolution rules.")
		return nil, errors.NewServerError(errors.ErrWhileFetchingUnificationRules, err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			logger.Debug("Error occurred while closing cursor.", err)
		}
	}(cursor, ctx)
	var rules []models.UnificationRule
	if err = cursor.All(ctx, &rules); err != nil {
		logger.Debug("Error occurred while decoding resolution rules.", err)
		return nil, errors.NewServerError(errors.ErrWhileFetchingUnificationRules, err)
	}
	logger.Info("Successfully fetched resolution rules")
	return rules, nil
}

// GetUnificationRule retrieves a specific resolution rule by rule_id.
func (repo *UnificationRuleRepository) GetUnificationRule(ruleId string) (models.UnificationRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"rule_id": ruleId}
	var rule models.UnificationRule

	err := repo.Collection.FindOne(ctx, filter).Decode(&rule)
	if err != nil {
		logger.Debug("Error occurred while fetching resolution rule with rule_id: "+ruleId, err)
		return rule, errors.NewServerError(errors.ErrWhileFetchingUnificationRule, err)
	}

	logger.Info("Successfully fetched resolution rule for rule_id: " + ruleId)
	return rule, nil
}

// GetUnificationRuleByPropertyName retrieves a specific resolution rule by property name.
func (repo *UnificationRuleRepository) GetUnificationRuleByPropertyName(ruleId string) (models.UnificationRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"rule_id": ruleId}
	var rule models.UnificationRule

	err := repo.Collection.FindOne(ctx, filter).Decode(&rule)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			logger.Info("No resolution rule found for rule_id: " + ruleId)
			return models.UnificationRule{}, nil
		}
		logger.Debug("Error occurred while fetching resolution rule with rule_id: "+ruleId, err)
		return models.UnificationRule{}, errors.NewServerError(errors.ErrWhileFetchingUnificationRule, err)
	}

	logger.Info("Successfully fetched resolution rule for rule_id: " + ruleId)
	return rule, nil
}

// PatchUnificationRule modifies specific fields
func (repo *UnificationRuleRepository) PatchUnificationRule(ruleId string, updates bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now().UTC().Unix()

	filter := bson.M{"rule_id": ruleId}
	update := bson.M{"$set": updates}

	_, err := repo.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileUpdatingUnificationRule, err)
	}
	logger.Info("Successfully updated resolution rule for rule_id: " + ruleId)
	return nil
}

// DeleteUnificationRule Removes a resolution rule.
func (repo *UnificationRuleRepository) DeleteUnificationRule(ruleId string) error {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	filter := bson.M{"rule_id": ruleId}
	_, err := repo.Collection.DeleteOne(ctx, filter)
	if err != nil {
		logger.Error(err, "Error while deleting resolution rule for rule_id: "+ruleId)
		return err
	}
	logger.Info("Successfully deleted resolution rule with rule_id: " + ruleId)
	return nil
}
