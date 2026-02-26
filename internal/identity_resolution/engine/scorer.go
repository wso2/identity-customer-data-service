/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package engine

import (
	"fmt"
	"math"

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

type fellegiSunterParams struct {
	m float64
	u float64
}

var defaultFSParams = map[string]fellegiSunterParams{
	constants.AttributeTypeEmail:    {m: 0.97, u: 0.001},
	constants.AttributeTypePhone:    {m: 0.95, u: 0.005},
	constants.AttributeTypeID:       {m: 0.99, u: 0.0005},
	constants.AttributeTypeName:     {m: 0.90, u: 0.05},
	constants.AttributeTypeLocation: {m: 0.85, u: 0.08},
	constants.AttributeTypeDate:     {m: 0.90, u: 0.05},
	constants.AttributeTypeGender:   {m: 0.98, u: 0.50},
	constants.AttributeTypeUnknown:  {m: 0.90, u: 0.05},
}

func ScoreCandidate(
	inputAttrs map[string]interface{},
	candidate *model.ProfileData,
	rules []*model.EnrichedRule,
	mode string,
	autoMergeThreshold float64,
) (float64, map[string]float64) {
	logger := log.GetLogger()

	breakdown := make(map[string]float64)
	weightedSum := 0.0

	n := len(rules)
	totalWeight := 0.0
	for i := range rules {
		totalWeight += float64(n - i)
	}

	matchedRules := 0

	fsLogScore := 0.0
	fsFieldsCompared := 0

	for i, rule := range rules {
		val1 := getStringValue(inputAttrs, rule.PropertyName)

		weight := float64(n - i)

		if val1 == "" {
			logger.Debug(fmt.Sprintf("Scorer: skipping rule '%s' — input has no value", rule.RuleName))
			continue
		}

		val2 := candidate.GetAttribute(rule.PropertyName)
		if val2 == "" {
			logger.Debug(fmt.Sprintf("Scorer: rule '%s' — candidate missing value (score=0, weight=%.0f)",
				rule.RuleName, weight))
			fsParams := getFSParams(rule.AttrType)
			fsContrib := math.Log2((1 - fsParams.m) / (1 - fsParams.u))
			fsLogScore += fsContrib
			fsFieldsCompared++
			logger.Info(fmt.Sprintf(
				"Scorer-FS: rule '%s' (%s) — MISSING value, disagree contrib=%.4f, m=%.4f, u=%.4f, runningLogScore=%.4f",
				rule.RuleName, rule.AttrType, fsContrib, fsParams.m, fsParams.u, fsLogScore))
			continue
		}

		score := MatchAttribute(val1, val2, rule.AttrType, mode, rule.ValueType)

		if score > 0 && score < rule.MinScore {
			logger.Debug(fmt.Sprintf("Scorer: rule '%s' score=%.4f below minScore=%.2f, treating as 0",
				rule.RuleName, score, rule.MinScore))
			score = 0.0
		}

		breakdown[rule.PropertyName] = score
		weightedSum += score * weight

		if score > 0 {
			matchedRules++
		}

		fsParams := getFSParams(rule.AttrType)
		var fsContrib float64
		if score >= rule.MinScore {
			effectiveM := fsParams.m * score
			effectiveU := fsParams.u
			if effectiveM > 0 && effectiveU > 0 {
				fsContrib = math.Log2(effectiveM / effectiveU)
				fsLogScore += fsContrib
			}
			logger.Info(fmt.Sprintf(
				"Scorer-FS: rule '%s' (%s) — AGREE, score=%.4f, effectiveM=%.4f, u=%.4f, log2(eM/u)=%.4f, runningLogScore=%.4f",
				rule.RuleName, rule.AttrType, score, effectiveM, effectiveU, fsContrib, fsLogScore))
		} else {
			fsContrib = math.Log2((1 - fsParams.m) / (1 - fsParams.u))
			fsLogScore += fsContrib
			logger.Info(fmt.Sprintf(
				"Scorer-FS: rule '%s' (%s) — DISAGREE, score=%.4f < minScore=%.2f, contrib=%.4f, (1-m)=%.4f, (1-u)=%.4f, runningLogScore=%.4f",
				rule.RuleName, rule.AttrType, score, rule.MinScore, fsContrib, 1-fsParams.m, 1-fsParams.u, fsLogScore))
		}
		fsFieldsCompared++

		logger.Info(fmt.Sprintf("Scorer: rule '%s' (%s) — matchScore=%.4f, weight=%.0f, minScore=%.2f, weightedSum=%.4f",
			rule.RuleName, rule.AttrType, score, weight, rule.MinScore, weightedSum))
	}

	if totalWeight == 0 {
		logger.Warn("Scorer: no rules — returning score 0.0",
			log.String("candidateID", candidate.ProfileID))
		return 0.0, breakdown
	}

	baseScore := weightedSum / totalWeight

	coverageRatio := float64(matchedRules) / float64(len(rules))
	boost := coverageRatio * constants.CoverageBoostFactor * (1.0 - baseScore)
	weightedScore := baseScore + boost

	fsScore := 0.0
	if fsFieldsCompared > 0 {
		fsScore = fellegiSunterToProb(fsLogScore)
	}

	logger.Info(fmt.Sprintf(
		"Scorer-FS SUMMARY: candidate '%s' — fieldsCompared=%d, totalLogScore=%.4f, sigmoid(logScore)=%.4f, weightedAvg=%.4f, baseScore=%.4f, boost=%.4f",
		candidate.ProfileID, fsFieldsCompared, fsLogScore, fsScore, weightedScore, baseScore, boost))

	finalScore := math.Max(weightedScore, fsScore)

	autoMergeCap := autoMergeThreshold - 0.001
	if baseScore < autoMergeThreshold && finalScore >= autoMergeThreshold {
		finalScore = math.Min(finalScore, autoMergeCap)
	}

	if finalScore > 1.0 {
		finalScore = 1.0
	}
	if finalScore < 0.0 {
		finalScore = 0.0
	}

	logger.Info(fmt.Sprintf(
		"Scorer: candidate '%s' — weighted=%.4f, FS=%.4f(log=%.2f), coverage=%.0f%% (%d/%d), final=%.4f, mode=%s",
		candidate.ProfileID, weightedScore, fsScore, fsLogScore,
		coverageRatio*100, matchedRules, len(rules),
		finalScore, mode))

	return finalScore, breakdown
}

func getFSParams(attrType string) fellegiSunterParams {
	if params, ok := defaultFSParams[attrType]; ok {
		return params
	}
	return defaultFSParams[constants.AttributeTypeUnknown]
}

func fellegiSunterToProb(logScore float64) float64 {
	return 1.0 / (1.0 + math.Pow(2, -logScore))
}
