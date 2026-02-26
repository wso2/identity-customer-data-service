// cmd/backfill_keys/main.go — one-shot tool to backfill blocking_keys for all profiles.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine"
	irModel "github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
)

func main() {
	dsn := "host=localhost port=5432 user=postgres password=cdspwd dbname=cds_db sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	org := "carbon.super"
	if len(os.Args) > 1 {
		org = os.Args[1]
	}

	// 1. Load unification rules → enriched rules
	rules, err := loadRules(db, org)
	if err != nil {
		log.Fatalf("loadRules: %v", err)
	}
	fmt.Printf("Loaded %d rules for org=%s\n", len(rules), org)

	// 2. Load all profiles
	rows, err := db.Query(`SELECT profile_id, identity_attributes, traits FROM profiles WHERE org_handle=$1`, org)
	if err != nil {
		log.Fatalf("query profiles: %v", err)
	}
	defer rows.Close()

	var totalInserted int
	for rows.Next() {
		var profileID string
		var iaJSON, trJSON []byte
		if err := rows.Scan(&profileID, &iaJSON, &trJSON); err != nil {
			log.Fatalf("scan: %v", err)
		}

		// Flatten identity_attributes and traits into a single map
		flatAttrs := make(map[string]interface{})
		var ia map[string]interface{}
		if err := json.Unmarshal(iaJSON, &ia); err == nil {
			for k, v := range ia {
				flatAttrs["identity_attributes."+k] = v
			}
		}
		var tr map[string]interface{}
		if err := json.Unmarshal(trJSON, &tr); err == nil {
			flattenNested("traits", tr, flatAttrs)
		}

		keys := engine.GenerateProfileBlockingKeys(rules, flatAttrs)
		if len(keys) == 0 {
			continue
		}

		// Upsert blocking keys
		for _, k := range keys {
			_, err := db.Exec(`INSERT INTO blocking_keys (profile_id, org_handle, attribute_name, key_value)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT DO NOTHING`,
				profileID, org, k.AttributeName, k.KeyValue)
			if err != nil {
				log.Printf("insert key for %s: %v", profileID, err)
				continue
			}
			totalInserted++
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows err: %v", err)
	}

	fmt.Printf("Backfilled %d blocking keys for org=%s\n", totalInserted, org)
}

func loadRules(db *sql.DB, org string) ([]*irModel.EnrichedRule, error) {
	rows, err := db.Query(`SELECT rule_name, property_name, priority FROM unification_rules WHERE org_handle=$1 AND is_active=true ORDER BY priority`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*irModel.EnrichedRule
	for rows.Next() {
		var name, prop string
		var priority int
		if err := rows.Scan(&name, &prop, &priority); err != nil {
			return nil, err
		}
		attrType := irModel.DetectAttributeType(prop)
		rules = append(rules, &irModel.EnrichedRule{
			RuleName:     name,
			PropertyName: prop,
			AttrType:     attrType,
			Priority:     priority,
			IsActive:     true,
		})
	}
	return rules, rows.Err()
}

func flattenNested(prefix string, m map[string]interface{}, out map[string]interface{}) {
	for k, v := range m {
		fullKey := prefix + "." + k
		switch vv := v.(type) {
		case map[string]interface{}:
			flattenNested(fullKey, vv, out)
		default:
			out[fullKey] = v
			_ = vv
		}
	}
}

// Helper to build value placeholders
func placeholders(n int) string {
	ph := make([]string, n)
	for i := range ph {
		ph[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(ph, ", ")
}
