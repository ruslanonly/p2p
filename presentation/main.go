package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"

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
	PeerHubs      []string               `json:"peer_hubs"`
	PeerAbonents  []string               `json:"peer_abonents"`
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

	var mu sync.Mutex

	r.POST("/report", func(ctx *gin.Context) {
		mu.Lock()
		handleReport(driver, ctx)
		mu.Unlock()
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
		_, err := tx.Run(ctx,
			`MERGE (a:Agent {pid: $id})
			 SET a.name = $name,
			     a.state = $state,
			     a.yellowReports = $yellowReports,
			     a.redReports = $redReports,
			     a.blockedHosts = $blockedHosts,
			     a.peerHubs = $peerHubs,
			     a.peerAbonents = $peerAbonents`,
			map[string]interface{}{
				"id":            report.ID,
				"name":          report.Name,
				"state":         report.State,
				"yellowReports": string(yellowReportsJSON),
				"redReports":    string(redReportsJSON),
				"blockedHosts":  netIPToStringSlice(report.BlockedHosts),
				"peerHubs":      report.PeerHubs,
				"peerAbonents":  report.PeerAbonents,
			})
		if err != nil {
			return nil, err
		}

		// Удаляем устаревшие IS_ABONENT_FOR связи
		_, err = tx.Run(ctx, `
			MATCH (a:Agent {pid: $id})-[r:IS_ABONENT_FOR]->(b:Agent)
			WHERE NOT (b.pid IN $peerHubs)
			DELETE r
		`, map[string]interface{}{
			"id":       report.ID,
			"peerHubs": report.PeerHubs,
		})
		if err != nil {
			return nil, err
		}

		// Удаляем устаревшие IS_HUB_FOR связи
		_, err = tx.Run(ctx, `
			MATCH (a:Agent {pid: $id})-[r:IS_HUB_FOR]->(b:Agent)
			WHERE NOT (b.pid IN $peerAbonents)
			DELETE r
		`, map[string]interface{}{
			"id":           report.ID,
			"peerAbonents": report.PeerAbonents,
		})
		if err != nil {
			return nil, err
		}

		// Добавляем связи к peerHubs
		for _, hubID := range report.PeerHubs {
			_, err := tx.Run(ctx,
				`MATCH (hub:Agent {pid: $hubID})
				 MATCH (a:Agent {pid: $agentID})
				 OPTIONAL MATCH (hub)-[r]->(a)
				 DELETE r
				 CREATE (hub)-[:IS_HUB_FOR]->(a)`,
				map[string]interface{}{
					"hubID":   hubID,
					"agentID": report.ID,
				})
			if err != nil {
				return nil, err
			}
		}

		// Добавляем связи к peerAbonents
		for _, abonentID := range report.PeerAbonents {
			_, err := tx.Run(ctx,
				`MATCH (abonent:Agent {pid: $abonentID})
				 MATCH (a:Agent {pid: $agentID})
				 OPTIONAL MATCH (abonent)-[r]->(a)
				 DELETE r
				 CREATE (abonent)-[:IS_ABONENT_FOR]->(a)`,
				map[string]interface{}{
					"abonentID": abonentID,
					"agentID":   report.ID,
				})
			if err != nil {
				return nil, err
			}
		}

		// // Объединение двухсторонних связей в HUB <-> HUB
		// _, err = tx.Run(ctx, `
		// 	MATCH (a:Agent)-[r1:IS_ABONENT_FOR]->(b:Agent)
		// 	MATCH (b)-[r2:IS_ABONENT_FOR]->(a)
		// 	WHERE id(r1) < id(r2)
		// 	DELETE r1, r2
		// 	CREATE (a)-[:HUB]->(b)
		// 	CREATE (b)-[:HUB]->(a)
		// `, nil)
		// if err != nil {
		// 	return nil, err
		// }

		return nil, nil
	})

	if err != nil {
		log.Println("Neo4j write error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func ensureAgentIDUniqueConstraint(driver neo4j.DriverWithContext) error {
	ctx := context.Background()

	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, `
			CREATE CONSTRAINT unique_agent_id IF NOT EXISTS
			FOR (a:Agent)
			REQUIRE a.pid IS UNIQUE
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
