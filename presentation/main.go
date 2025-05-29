package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type AgentReportNeighbour struct {
	ID    string `json:"id"`
	IsHub bool   `json:"is_hub"`
}

type AgentReport struct {
	ID            string                 `json:"agent_id"`
	Name          string                 `json:"name"`
	State         string                 `json:"state"`
	Neighbors     []AgentReportNeighbour `json:"neighbors"`
	YellowReports map[string][]peer.ID   `json:"yellow_reports"`
	RedReports    map[string][]peer.ID   `json:"red_reports"`
	BlockedHosts  []net.IP               `json:"blocked_reports"`
}

func netIPToStringSlice(ips []net.IP) []string {
	strs := make([]string, 0, len(ips))
	for _, ip := range ips {
		strs = append(strs, ip.String())
	}
	return strs
}

func main() {
	driver, err := neo4j.NewDriverWithContext(
		"neo4j://172.20.0.2:7687",
		neo4j.BasicAuth("neo4j", "password", ""),
	)
	if err != nil {
		log.Fatalf("Failed to create driver: %v", err)
	}
	defer driver.Close(context.Background())

	ensureAgentIDUniqueConstraint(driver)

	r := gin.Default()
	r.POST("/report", func(ctx *gin.Context) {
		handleReport(driver, ctx)
	})

	log.Println("Server running on :8080")
	r.Run(":8080")
}

func handleReport(driver neo4j.DriverWithContext, c *gin.Context) {
	var report AgentReport
	if err := c.ShouldBindJSON(&report); err != nil {
		log.Println("JSON ERROR:", err, c.Request.Body)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	yellowReportsJSON, err := json.Marshal(report.YellowReports)
	if err != nil {
		log.Println("Failed to serialize yellowReports:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize yellowReports"})
		return
	}
	redReportsJSON, err := json.Marshal(report.RedReports)
	if err != nil {
		log.Println("Failed to serialize redReports:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize redReports"})
		return
	}

	ctx := context.Background()
	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// Собираем ID всех соседей
		neighbourIDs := map[string]bool{}
		for _, n := range report.Neighbors {
			neighbourIDs[n.ID] = true
		}

		_, err := tx.Run(ctx,
			`MERGE (a:Agent {id: $id})
			 SET a.name = $name, a.state = $state, a.yellowReports = $yellowReports, a.redReports = $redReports, a.blockedHosts = $blockedHosts`,
			map[string]interface{}{
				"id":            report.ID,
				"name":          report.Name,
				"state":         report.State,
				"yellowReports": string(yellowReportsJSON),
				"redReports":    string(redReportsJSON),
				"blockedHosts":  netIPToStringSlice(report.BlockedHosts),
			},
		)
		if err != nil {
			return nil, err
		}

		// Удаляем все исходящие связи, которых нет в новых данных
		_, err = tx.Run(ctx, `
			MATCH (a:Agent {id: $id})-[r]->(other:Agent)
			WHERE NOT other.id IN $neighbours
			DELETE r`,
			map[string]interface{}{"id": report.ID, "neighbours": keys(neighbourIDs)},
		)
		if err != nil {
			return nil, err
		}

		// Обновляем или создаём соседей и связи
		for _, neighbour := range report.Neighbors {
			relationship := "IS_HUB_FOR"
			if neighbour.IsHub {
				relationship = "IS_ABONENT_FOR"
			}

			_, err = tx.Run(ctx,
				fmt.Sprintf(`
				MATCH (a:Agent {id: $agent})
				MATCH (b:Agent {id: $neighbour})
				OPTIONAL MATCH (a)-[r]->(b)
				DELETE r
				CREATE (a)-[:%s]->(b)`, relationship),
				map[string]interface{}{
					"agent":     report.ID,
					"neighbour": neighbour.ID,
				},
			)
			if err != nil {
				return nil, err
			}
		}

		_, err = tx.Run(ctx, `
			MATCH (a:Agent)-[r1:IS_ABONENT_FOR]->(b:Agent)
			MATCH (b)-[r2:IS_ABONENT_FOR]->(a)
			WHERE id(r1) < id(r2)
			WITH a, b, r1, r2
			DELETE r1, r2
			CREATE (a)-[:HUB]->(b)
			CREATE (b)-[:HUB]->(a)
		`, nil)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	if err != nil {
		log.Println("Neo4j write error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func keys(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func ensureAgentIDUniqueConstraint(driver neo4j.DriverWithContext) error {
	ctx := context.Background()

	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, `
			CREATE CONSTRAINT unique_agent_id IF NOT EXISTS
			FOR (a:Agent)
			REQUIRE a.id IS UNIQUE
		`, nil)
		return nil, err
	})

	if err != nil {
		log.Println("Failed to ensure uniqueness constraint:", err)
		return err
	}

	log.Println("Agent ID uniqueness constraint ensured.")
	return nil
}
