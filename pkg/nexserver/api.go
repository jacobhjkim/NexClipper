/*
Copyright 2019 NexClipper.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nexserver

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *NexServer) SetupApiHandler() {
	gin.SetMode("release")
	router := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"*"}
	config.AllowHeaders = []string{"*"}
	config.AllowCredentials = true

	router.Use(cors.New(config))

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", s.ApiHealth)
		v1.GET("/clusters", s.ApiClusterList)
		v1.GET("/agents", s.ApiAgentListAll)
		v1.GET("/nodes", s.ApiNodeListAll)
		v1.GET("/metric_names", s.ApiMetricNameList)
		v1.GET("/status", s.ApiStatus)
	}

	clusters := v1.Group("/clusters")
	{
		clusters.GET("/:clusterId/agents", s.ApiAgentList)
		clusters.GET("/:clusterId/nodes", s.ApiNodeList)
	}
	snapshot := v1.Group("/snapshot")
	{
		snapshot.GET("/:clusterId/nodes", s.ApiSnapshotNodes)
		snapshot.GET("/:clusterId/nodes/:nodeId", s.ApiSnapshotNodes)
		snapshot.GET("/:clusterId/nodes/:nodeId/processes", s.ApiSnapshotProcesses)
		snapshot.GET("/:clusterId/nodes/:nodeId/processes/:processId", s.ApiSnapshotProcesses)
		snapshot.GET("/:clusterId/nodes/:nodeId/containers", s.ApiSnapshotContainers)
		snapshot.GET("/:clusterId/nodes/:nodeId/containers/:containerId", s.ApiSnapshotContainers)
		snapshot.GET("/:clusterId/k8s/pods", s.ApiSnapshotPods)
		snapshot.GET("/:clusterId/k8s/namespaces/:namespaceId/pods", s.ApiSnapshotPods)
		snapshot.GET("/:clusterId/k8s/namespaces/:namespaceId/pods/:podId", s.ApiSnapshotPods)
	}
	metrics := v1.Group("/metrics")
	{
		metrics.GET("/:clusterId/nodes", s.ApiMetricsNodes)
		metrics.GET("/:clusterId/nodes/:nodeId", s.ApiMetricsNodes)
		metrics.GET("/:clusterId/nodes/:nodeId/processes", s.ApiMetricsProcesses)
		metrics.GET("/:clusterId/nodes/:nodeId/processes/:processId", s.ApiMetricsProcesses)
		metrics.GET("/:clusterId/nodes/:nodeId/containers", s.ApiMetricsContainers)
		metrics.GET("/:clusterId/nodes/:nodeId/containers/:containerId", s.ApiMetricsContainers)
		metrics.GET("/:clusterId/k8s/pods", s.ApiMetricsPods)
		metrics.GET("/:clusterId/k8s/namespaces/:namespaceId/pods", s.ApiMetricsPods)
		metrics.GET("/:clusterId/k8s/namespaces/:namespaceId/pods/:podId", s.ApiMetricsPods)
		metrics.GET("/:clusterId/summary", s.ApiMetricsClusterSummary)
	}
	summary := v1.Group("/summary")
	{
		summary.GET("/clusters", s.ApiSummaryClusters)
		summary.GET("/clusters/:clusterId", s.ApiSummaryClusters)
		summary.GET("/clusters/:clusterId/nodes", s.ApiSummaryNodes)
		summary.GET("/clusters/:clusterId/nodes/:nodeId", s.ApiSummaryNodes)
	}
	incident := v1.Group("/incidents")
	{
		incident.GET("/basic", s.ApiIncidentBasic)
	}

	go func() {
		err := router.Run(fmt.Sprintf("%s:%d", s.config.Server.BindAddress, s.config.Server.ApiPort))
		if err != nil {
			log.Printf("failed api handler: %v\n", err)
		}
	}()
}

func (s *NexServer) ApiResponseJson(c *gin.Context, code int, status, message string) {
	c.JSON(code, gin.H{
		"status":  status,
		"message": message,
	})
}

func (s *NexServer) Param(c *gin.Context, key string) string {
	return s.RemoveSpecialChar(c.Param(key))
}

func (s *NexServer) RemoveSpecialChar(key string) string {
	chars := []string{"'", "\""}

	for _, ch := range chars {
		key = strings.ReplaceAll(key, ch, "")
	}

	return key
}

func (s *NexServer) ApiStatus(c *gin.Context) {
	uptime := time.Since(s.serverStartTs)
	uptimeSeconds := uptime.Seconds()

	metricsPerSeconds := float64(s.metricSaveCounter) / uptimeSeconds

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data": gin.H{
			"uptime":            uptime.String(),
			"metricsPerSeconds": fmt.Sprintf("%.2f", metricsPerSeconds),
			"totalMetrics":      fmt.Sprintf("%d", s.metricSaveCounter),
		},
	})
}

type Query struct {
	Timezone    string   `json:"timezone"`
	MetricNames []string `json:"metricNames"`
	DateRange   []string `json:"dateRange"`
	Granularity string   `json:"granularity"`
}

func (s *NexServer) ParseQuery(c *gin.Context) *Query {
	var query Query

	queryParam := c.DefaultQuery("query", "")
	if queryParam != "" {
		err := json.Unmarshal([]byte(queryParam), &query)
		if err != nil {
			return nil
		}

		return &query
	}

	query.Timezone = s.RemoveSpecialChar(c.DefaultQuery("timezone", "UTC"))
	query.Granularity = s.RemoveSpecialChar(c.DefaultQuery("granularity", ""))
	query.DateRange = c.QueryArray("dateRange")
	query.MetricNames = c.QueryArray("metricNames")

	for idx, dateRange := range query.DateRange {
		query.DateRange[idx] = s.RemoveSpecialChar(dateRange)
	}
	for idx, metricName := range query.MetricNames {
		query.MetricNames[idx] = s.RemoveSpecialChar(metricName)
	}

	_, err := time.LoadLocation(query.Timezone)
	if err != nil {
		log.Printf("invalid timezone: %s: %v\n", query.Timezone, err)
		return nil
	}

	return &query
}

func (s *NexServer) ApiHealth(c *gin.Context) {
	err := s.db.DB().Ping()
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", "DB connection failed")
	} else {
		s.ApiResponseJson(c, 200, "ok", "")
	}
}

func (s *NexServer) ApiMetricNameList(c *gin.Context) {
	query := s.db.Raw(`
SELECT metric_names.id, metric_names.name, metric_names.help, metric_types.name as metric_type
FROM metric_names, metric_types
WHERE metric_names.type_id=metric_types.id`)
	rows, err, queryTime := s.QueryRowsWithTime(query)
	if err != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get metric names: %v\n", err))
		return
	}

	type MetricNameItem struct {
		Id   uint   `json:"id"`
		Name string `json:"name"`
		Help string `json:"help"`
		Type string `json:"type"`
	}
	metricNames := make([]MetricNameItem, 0, 16)

	for rows.Next() {
		metricNameItem := MetricNameItem{}

		err := rows.Scan(&metricNameItem.Id, &metricNameItem.Name, &metricNameItem.Help, &metricNameItem.Type)
		if err != nil {
			log.Printf("failed to get record from metrics_names: %v", err)
			continue
		}

		metricNames = append(metricNames, metricNameItem)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          metricNames,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiSummaryClusters(c *gin.Context) {
	targetClusterId := s.Param(c, "clusterId")
	clusterQuery := ""
	if targetClusterId != "" {
		clusterQuery = fmt.Sprintf(" AND m2.cluster_id=%s", targetClusterId)
	}

	q := fmt.Sprintf(`
SELECT m1.cluster_id, clusters.name, metric_names.name, ROUND(SUM(m1.value))
FROM metric_names, metric_labels, nodes, clusters, metrics m1
JOIN (
    SELECT m2.node_id, MAX(ts) ts
    FROM metrics m2
    WHERE m2.ts >= NOW() - interval '60 seconds'
      AND m2.process_id=0
      AND m2.container_id=0 %s
    GROUP BY m2.node_id) newest
ON newest.node_id=m1.node_id AND newest.ts=m1.ts
WHERE m1.name_id=metric_names.id
  AND m1.node_id=nodes.id
  AND m1.label_id=metric_labels.id
  AND m1.process_id=0
  AND m1.container_id=0
  AND m1.cluster_id=clusters.id
GROUP BY m1.cluster_id, clusters.name, metric_names.name`, clusterQuery)

	rows, err := s.db.Raw(q).Rows()
	if err != nil {
		log.Printf("failed to get data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	items := make(map[uint]map[string]float64)
	var clusterId uint
	var clusterName string
	var metricName string
	var value float64
	for rows.Next() {
		err := rows.Scan(&clusterId, &clusterName, &metricName, &value)
		if err != nil {
			log.Printf("failed to get data: %v", err)
			continue
		}

		clusterMetrics, found := items[clusterId]
		if !found {
			clusterMetrics = make(map[string]float64)
			items[clusterId] = clusterMetrics
		}

		clusterMetrics[metricName] = value
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    items,
	})
}

func (s *NexServer) ApiSummaryNodes(c *gin.Context) {
	targetClusterId := s.Param(c, "clusterId")
	if targetClusterId == "" {
		s.ApiResponseJson(c, 404, "bad", "missing parameters")
		return
	}

	q := fmt.Sprintf(`
SELECT m1.node_id, nodes.host, metric_names.name, ROUND(SUM(m1.value), 2)
FROM metric_names, metric_labels, nodes, metrics m1
JOIN (
    SELECT m2.node_id, MAX(ts) ts
    FROM metrics m2
    WHERE m2.ts >= NOW() - interval '60 seconds'
      AND m2.process_id=0
      AND m2.container_id=0
      AND m2.cluster_id=%s
    GROUP BY m2.node_id) newest
ON newest.node_id=m1.node_id AND newest.ts=m1.ts
WHERE m1.name_id=metric_names.id
  AND m1.node_id=nodes.id
  AND m1.label_id=metric_labels.id
  AND m1.process_id=0
  AND m1.container_id=0
GROUP BY m1.node_id, nodes.host, metric_names.name`, targetClusterId)

	rows, err := s.db.Raw(q).Rows()
	if err != nil {
		log.Printf("failed to get data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	items := make(map[string]map[string]float64)
	var hostId uint
	var host string
	var metricName string
	var value float64
	for rows.Next() {
		err := rows.Scan(&hostId, &host, &metricName, &value)
		if err != nil {
			log.Printf("failed to get data: %v", err)
			continue
		}

		nodeMetrics, found := items[host]
		if !found {
			nodeMetrics = make(map[string]float64)
			items[host] = nodeMetrics
		}

		nodeMetrics[metricName] = value
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    items,
	})
}

func (s *NexServer) ApiClusterList(c *gin.Context) {
	query := s.db.Raw(`
SELECT clusters.id as cluster_id, clusters.name, 
       coalesce(k8s_clusters.id::integer, 0) as k8s_agent_cluster_id
FROM clusters
LEFT JOIN k8s_clusters ON clusters.id=k8s_clusters.agent_cluster_id`)
	rows, err, queryTime := s.QueryRowsWithTime(query)
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type ClusterItem struct {
		Id         uint   `json:"id"`
		Name       string `json:"name"`
		Kubernetes bool   `json:"kubernetes"`
	}
	items := make([]ClusterItem, 0, 16)

	for rows.Next() {
		var clusterItem ClusterItem
		var k8sAgentClusterId uint

		err := rows.Scan(&clusterItem.Id, &clusterItem.Name, &k8sAgentClusterId)
		if err != nil {
			log.Printf("failed to get data: %v", err)
			continue
		}

		if k8sAgentClusterId == 0 {
			clusterItem.Kubernetes = false
		} else {
			clusterItem.Kubernetes = true
		}

		items = append(items, clusterItem)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          items,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiAgentList(c *gin.Context) {
	cId := s.Param(c, "clusterId")
	if cId == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid cluster id")
		return
	}

	var agents []Agent

	queryStart := time.Now()
	result := s.db.Where("cluster_id=?", cId).Find(&agents)
	queryTime := time.Since(queryStart)
	if result.Error != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", result.Error))
		return
	}

	type AgentItem struct {
		Id      uint   `json:"id"`
		Version string `json:"version"`
		Ip      string `json:"ip"`
		Online  bool   `json:"online"`
	}
	items := make([]AgentItem, 0, 16)

	for _, agent := range agents {
		items = append(items, AgentItem{
			Id:      agent.ID,
			Version: agent.Version,
			Ip:      agent.Ipv4,
			Online:  agent.Online,
		})
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          items,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiAgentListAll(c *gin.Context) {
	query := s.db.Table("agents").
		Select("agents.id, agents.version, agents.ipv4, agents.online, clusters.name").
		Joins("left join clusters on agents.cluster_id=clusters.id")
	rows, err, queryTime := s.QueryRowsWithTime(query)
	if err != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type AgentItem struct {
		Id      uint   `json:"id"`
		Version string `json:"version"`
		Ip      string `json:"ip"`
		Online  bool   `json:"online"`
	}
	clusterMap := make(map[string][]*AgentItem)

	var clusterName string
	for rows.Next() {
		var agentItem AgentItem

		err := rows.Scan(&agentItem.Id, &agentItem.Version, &agentItem.Ip, &agentItem.Online, &clusterName)
		if err != nil {
			continue
		}
		_, found := clusterMap[clusterName]
		if !found {
			clusterMap[clusterName] = make([]*AgentItem, 0)
		}

		items := clusterMap[clusterName]
		items = append(items, &agentItem)
		clusterMap[clusterName] = items
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          clusterMap,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiNodeList(c *gin.Context) {
	cId := s.Param(c, "clusterId")
	if cId == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid cluster id")
		return
	}

	var nodes []Node

	queryStart := time.Now()
	result := s.db.Where("cluster_id=?", cId).Find(&nodes)
	queryTime := time.Since(queryStart)
	if result.Error != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", result.Error))
		return
	}

	type NodeItem struct {
		Id              uint   `json:"id"`
		Host            string `json:"host"`
		Ip              string `json:"ip"`
		Os              string `json:"os"`
		Platform        string `json:"platform"`
		PlatformFamily  string `json:"platform_family"`
		PlatformVersion string `json:"platform_version"`
		AgentId         uint   `json:"agent_id"`
	}

	items := make([]NodeItem, 0, 16)
	for _, node := range nodes {
		items = append(items, NodeItem{
			Id:              node.ID,
			Host:            node.Host,
			Ip:              node.Ipv4,
			Os:              node.Os,
			Platform:        node.Platform,
			PlatformFamily:  node.PlatformFamily,
			PlatformVersion: node.PlatformVersion,
			AgentId:         node.AgentID,
		})
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          items,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiNodeListAll(c *gin.Context) {
	query := s.db.Table("nodes").
		Select("nodes.id, nodes.host, nodes.ipv4, nodes.os, " +
			"nodes.platform, nodes.platform_family, nodes.platform_version, nodes.agent_id, clusters.name").
		Joins("left join clusters on nodes.cluster_id=clusters.id")
	rows, err, queryTime := s.QueryRowsWithTime(query)
	if err != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type NodeItem struct {
		Id              uint   `json:"id"`
		Host            string `json:"host"`
		Ip              string `json:"ip"`
		Os              string `json:"os"`
		Platform        string `json:"platform"`
		PlatformFamily  string `json:"platform_family"`
		PlatformVersion string `json:"platform_version"`
		AgentId         uint   `json:"agent_id"`
	}
	clusterMap := make(map[string][]*NodeItem)

	var clusterName string
	for rows.Next() {
		nodeItem := NodeItem{}
		err := rows.Scan(&nodeItem.Id, &nodeItem.Host, &nodeItem.Ip,
			&nodeItem.Os, &nodeItem.Platform, &nodeItem.PlatformFamily, &nodeItem.PlatformVersion,
			&nodeItem.AgentId, &clusterName)
		if err != nil {
			continue
		}
		_, found := clusterMap[clusterName]
		if !found {
			clusterMap[clusterName] = make([]*NodeItem, 0)
		}

		items := clusterMap[clusterName]
		items = append(items, &nodeItem)
		clusterMap[clusterName] = items
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          clusterMap,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiSnapshotNodes(c *gin.Context) {
	cId := s.Param(c, "clusterId")
	if cId == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid cluster id")
		return
	}

	nodeId := s.Param(c, "nodeId")
	nodeQuery := ""
	if nodeId != "" {
		nodeQuery = fmt.Sprintf("AND m2.node_id=%s", nodeId)
	}

	query := s.ParseQuery(c)
	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND m2.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	q := fmt.Sprintf(`
SELECT nodes.host as node, nodes.id, m1.ts, ROUND(m1.value, 2), metric_names.name, metric_labels.label
FROM metric_names, metric_labels, nodes, metrics m1
JOIN (
    SELECT m2.node_id, m2.name_id, MAX(ts) ts
    FROM metrics m2
    WHERE m2.process_id=0 
        AND m2.container_id=0
		AND m2.ts >= NOW() - interval '60 seconds' %s %s
    GROUP BY m2.node_id, m2.name_id) newest
ON newest.node_id=m1.node_id AND newest.name_id=m1.name_id AND newest.ts=m1.ts
WHERE m1.name_id=metric_names.id 
	AND m1.node_id=nodes.id 
	AND m1.label_id=metric_labels.id`, nodeQuery, metricNameQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(q))
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type NodeMetric struct {
		Node        string    `json:"node"`
		NodeId      uint      `json:"node_id"`
		Ts          time.Time `json:"ts"`
		Value       float64   `json:"value"`
		MetricName  string    `json:"metric_name"`
		MetricLabel string    `json:"metric_label"`
	}

	results := make(map[string][]NodeMetric)

	for rows.Next() {
		var nodeMetric NodeMetric

		err := rows.Scan(&nodeMetric.Node, &nodeMetric.NodeId, &nodeMetric.Ts, &nodeMetric.Value,
			&nodeMetric.MetricName, &nodeMetric.MetricLabel)
		if err != nil {
			continue
		}

		nodeMetrics, found := results[nodeMetric.Node]
		if !found {
			results[nodeMetric.Node] = make([]NodeMetric, 0, 16)
		}

		nodeMetrics = append(nodeMetrics, nodeMetric)
		results[nodeMetric.Node] = nodeMetrics
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) findMetricIdByNames(names []string) []string {
	if len(names) == 0 {
		return []string{}
	}

	quotedNames := make([]string, 0, len(names))
	for _, name := range names {
		quotedNames = append(quotedNames, fmt.Sprintf("'%s'", name))
	}
	namesQuery := strings.Join(quotedNames, ",")
	q := fmt.Sprintf("SELECT id FROM metric_names WHERE name IN (%s)", namesQuery)

	rows, err := s.db.Raw(q).Rows()
	if err != nil {
		log.Printf("failed to get metric names: %v", err)
		return []string{}
	}

	results := make([]string, 0, 4)
	var id string

	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			log.Printf("failed to get metric names id: %v", err)
			continue
		}

		results = append(results, id)
	}

	return results
}

func (s *NexServer) ApiMetricsNodes(c *gin.Context) {
	nodeId := s.Param(c, "nodeId")
	nodeQuery := ""
	if nodeId != "" {
		nodeQuery = fmt.Sprintf("AND metrics.node_id=%s", nodeId)
	}

	cId := s.Param(c, "clusterId")
	query := s.ParseQuery(c)
	if s.IsValidParams(cId, query, true, true) == false {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}

	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND metrics.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	truncateQuery := s.calculateGranularity(query.DateRange, query.Timezone, query.Granularity)

	metricQuery := fmt.Sprintf(`
SELECT nodes.host as node, nodes.id as node_id, ROUND(value, 2), bucket,
       metric_names.name, metric_labels.label FROM
    (SELECT metrics.node_id as node_id, avg(value) as value,
            metrics.name_id, metrics.label_id, %s
    FROM metrics
    WHERE ts >= '%s' AND ts < '%s' AND metrics.cluster_id=%s 
      AND metrics.process_id=0
      AND metrics.container_id=0 %s %s
    GROUP BY bucket, metrics.node_id, metrics.name_id, metrics.label_id)
        as metrics_bucket, nodes, metric_names, metric_labels
WHERE
    metrics_bucket.node_id=nodes.id AND
    metrics_bucket.name_id=metric_names.id AND
    metrics_bucket.label_id=metric_labels.id
ORDER BY bucket`, truncateQuery, query.DateRange[0], query.DateRange[1],
		cId, nodeQuery, metricNameQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(metricQuery))

	if err != nil {
		log.Printf("failed to get metric data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("unexpected error: %v", err))
		return
	}

	type MetricItem struct {
		Node        string  `json:"node"`
		NodeId      uint    `json:"node_id"`
		Value       float64 `json:"value"`
		Bucket      string  `json:"bucket"`
		MetricName  string  `json:"metric_name"`
		MetricLabel string  `json:"metric_label"`
	}
	results := make([]MetricItem, 0, 16)

	for rows.Next() {
		var item MetricItem

		err := rows.Scan(&item.Node, &item.NodeId, &item.Value, &item.Bucket, &item.MetricName, &item.MetricLabel)
		if err != nil {
			log.Printf("failed to get record: %v", err)
			continue
		}

		results = append(results, item)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"count":         len(results),
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) CheckRequiredParams(c *gin.Context, params []string) (map[string]string, bool) {
	required := make(map[string]string)

	for _, param := range params {
		value := s.Param(c, param)
		if value == "" {
			return nil, false
		}

		required[param] = value
	}

	return required, true
}

func (s *NexServer) IsValidParams(clusterId string, query *Query, existDateRange bool, existMetricNames bool) bool {
	if clusterId == "" || query == nil {
		return false
	}

	if existDateRange {
		if query.DateRange == nil || len(query.DateRange) < 2 {
			return false
		}
	}

	if existMetricNames {
		if query.MetricNames == nil || len(query.MetricNames) < 1 {
			return false
		}
	}

	return true
}

func (s *NexServer) ApiSnapshotProcesses(c *gin.Context) {
	params, ok := s.CheckRequiredParams(c, []string{"clusterId", "nodeId"})
	if !ok {
		s.ApiResponseJson(c, 404, "bad", "missing parameters")
		return
	}
	clusterId := params["clusterId"]
	nodeId := params["nodeId"]

	processId := s.Param(c, "processId")
	processQuery := ""
	if processId != "" {
		processQuery = fmt.Sprintf("AND m2.process_id=%s", processId)
	}

	query := s.ParseQuery(c)
	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND m2.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	q := fmt.Sprintf(`
SELECT m1.process_id, processes.name as process_name, m1.ts, ROUND(m1.value), metric_names.name, metric_labels.label
FROM metric_names, metric_labels, processes, metrics m1
JOIN (
    SELECT m2.process_id, MAX(ts) ts, name_id
    FROM metrics m2
    WHERE m2.ts >= NOW() - interval '60 seconds'
      AND m2.cluster_id=%s
      AND m2.node_id=%s %s %s
      AND m2.container_id=0
    GROUP BY m2.process_id, m2.name_id) newest
ON newest.process_id=m1.process_id AND newest.ts=m1.ts AND newest.name_id=m1.name_id
WHERE m1.name_id=metric_names.id
  AND m1.label_id=metric_labels.id
  AND m1.process_id=processes.id`, clusterId, nodeId, processQuery, metricNameQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(q))
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type ProcessMetric struct {
		Process     string    `json:"process"`
		ProcessId   uint      `json:"process_id"`
		Ts          time.Time `json:"ts"`
		Value       float64   `json:"value"`
		MetricName  string    `json:"metric_name"`
		MetricLabel string    `json:"metric_label"`
	}

	results := make(map[string][]ProcessMetric)

	for rows.Next() {
		var processMetric ProcessMetric

		err := rows.Scan(&processMetric.ProcessId, &processMetric.Process,
			&processMetric.Ts, &processMetric.Value,
			&processMetric.MetricName, &processMetric.MetricLabel)
		if err != nil {
			continue
		}

		processMetrics, found := results[processMetric.Process]
		if !found {
			results[processMetric.Process] = make([]ProcessMetric, 0, 16)
		}

		processMetrics = append(processMetrics, processMetric)
		results[processMetric.Process] = processMetrics
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiSnapshotContainers(c *gin.Context) {
	params, ok := s.CheckRequiredParams(c, []string{"clusterId", "nodeId"})
	if !ok {
		s.ApiResponseJson(c, 404, "bad", "missing parameters")
		return
	}
	clusterId := params["clusterId"]
	nodeId := params["nodeId"]

	containerId := s.Param(c, "containerId")
	containerQuery := ""
	if containerId != "" {
		containerQuery = fmt.Sprintf("AND m2.container_id=%s", containerId)
	}

	query := s.ParseQuery(c)
	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND m2.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	q := fmt.Sprintf(`
SELECT m1.container_id, containers.name as container_name, m1.ts, ROUND(m1.value), 
	metric_names.name, metric_labels.label
FROM metric_names, metric_labels, containers, metrics m1
JOIN (
    SELECT m2.container_id, name_id, MAX(ts) ts
    FROM metrics m2
    WHERE m2.ts >= NOW() - interval '60 seconds'
      AND m2.cluster_id=%s
      AND m2.node_id=%s %s %s
      AND m2.process_id=0
    GROUP BY m2.container_id, m2.name_id) newest
ON newest.container_id=m1.container_id AND newest.ts=m1.ts AND newest.name_id=m1.name_id
WHERE m1.name_id=metric_names.id
  AND m1.label_id=metric_labels.id
  AND m1.container_id=containers.id`, clusterId, nodeId, containerQuery, metricNameQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(q))
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type ContainerMetric struct {
		Container   string    `json:"container"`
		ContainerId uint      `json:"container_id"`
		Ts          time.Time `json:"ts"`
		Value       float64   `json:"value"`
		MetricName  string    `json:"metric_name"`
		MetricLabel string    `json:"metric_label"`
	}

	results := make(map[string][]ContainerMetric)

	for rows.Next() {
		var containerMetric ContainerMetric

		err := rows.Scan(&containerMetric.ContainerId, &containerMetric.Container,
			&containerMetric.Ts, &containerMetric.Value,
			&containerMetric.MetricName, &containerMetric.MetricLabel)
		if err != nil {
			continue
		}

		containerMetrics, found := results[containerMetric.Container]
		if !found {
			results[containerMetric.Container] = make([]ContainerMetric, 0, 16)
		}

		containerMetrics = append(containerMetrics, containerMetric)
		results[containerMetric.Container] = containerMetrics
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiSnapshotPods(c *gin.Context) {
	params, ok := s.CheckRequiredParams(c, []string{"clusterId"})
	if !ok {
		s.ApiResponseJson(c, 404, "bad", "missing parameters")
		return
	}
	clusterId := params["clusterId"]

	namespaceId := s.Param(c, "namespaceId")
	namespaceQuery := ""
	if namespaceId != "" {
		namespaceQuery = fmt.Sprintf(" AND k8s_namespaces.id=%s", namespaceId)
	}

	podId := s.Param(c, "podId")
	podQuery := ""
	if podId != "" {
		podQuery = fmt.Sprintf("   AND k8s_pods.id=%s", podId)
	}

	query := s.ParseQuery(c)
	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND m2.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	q := fmt.Sprintf(`
SELECT k8s_pods.name as pod, k8s_namespaces.name as namespace, m1.ts, ROUND(SUM(m1.value)) as value,
	metric_names.name as metric_name
FROM metric_names, containers, k8s_pods, k8s_containers, k8s_namespaces, metrics as m1
JOIN (
    SELECT m2.container_id, name_id, MAX(ts) ts
    FROM metrics m2
    WHERE m2.ts >= NOW() - interval '60 seconds'
      AND m2.cluster_id=%s
      AND m2.container_id != 0
      AND m2.process_id=0 %s
    GROUP BY m2.container_id, m2.name_id) newest
ON newest.container_id=m1.container_id AND newest.ts=m1.ts AND newest.name_id=m1.name_id
WHERE m1.name_id=metric_names.id
  AND m1.container_id=containers.id
  AND containers.container_id=k8s_containers.container_id
  AND k8s_containers.k8s_pod_id=k8s_pods.id
  AND k8s_pods.k8s_namespace_id=k8s_namespaces.id %s %s
GROUP BY pod, namespace, m1.ts, metric_name`, clusterId, metricNameQuery, namespaceQuery, podQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(q))
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type PodMetric struct {
		Pod        string    `json:"pod"`
		Namespace  string    `json:"namespace"`
		Ts         time.Time `json:"ts"`
		Value      float64   `json:"value"`
		MetricName string    `json:"metric_name"`
	}

	results := make(map[string][]PodMetric)

	for rows.Next() {
		var podMetric PodMetric

		err := rows.Scan(&podMetric.Pod, &podMetric.Namespace, &podMetric.Ts, &podMetric.Value, &podMetric.MetricName)
		if err != nil {
			continue
		}

		podMetrics, found := results[podMetric.Pod]
		if !found {
			results[podMetric.Pod] = make([]PodMetric, 0, 16)
		}

		podMetrics = append(podMetrics, podMetric)
		results[podMetric.Pod] = podMetrics
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiMetricsProcesses(c *gin.Context) {
	nodeId := s.Param(c, "nodeId")
	nodeQuery := ""
	if nodeId != "" {
		nodeQuery = fmt.Sprintf(" AND metrics.node_id=%s", nodeId)
	}

	processId := s.Param(c, "processId")
	processQuery := ""
	if processId != "" {
		processQuery = fmt.Sprintf(" AND metrics.process_id=%s", processId)
	}

	cId := s.Param(c, "clusterId")
	query := s.ParseQuery(c)
	if s.IsValidParams(cId, query, true, true) == false {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}

	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND metrics.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	truncateQuery := s.calculateGranularity(query.DateRange, query.Timezone, query.Granularity)

	q := fmt.Sprintf(`
SELECT processes.name as process, processes.id, ROUND(value, 2), bucket,
       metric_names.name, metric_labels.label FROM
    (SELECT metrics.process_id as process_id, avg(value) as value,
            metrics.name_id, metrics.label_id, %s
    FROM metrics
    WHERE ts >= '%s' AND ts < '%s'
      AND metrics.cluster_id=%s %s %s %s
    GROUP BY bucket, metrics.process_id, metrics.name_id, metrics.label_id)
        as metrics_bucket, metric_names, metric_labels, processes
WHERE
    metrics_bucket.process_id=processes.id AND
      metrics_bucket.name_id=metric_names.id AND
      metrics_bucket.label_id=metric_labels.id
ORDER BY bucket`, truncateQuery, query.DateRange[0], query.DateRange[1],
		cId, nodeQuery, processQuery, metricNameQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(q))

	if err != nil {
		log.Printf("failed to get metric data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("unexpected error: %v", err))
		return
	}

	type MetricItem struct {
		Process     string  `json:"process"`
		ProcessId   uint    `json:"process_id"`
		Value       float64 `json:"value"`
		Bucket      string  `json:"bucket"`
		MetricName  string  `json:"metric_name"`
		MetricLabel string  `json:"metric_label"`
	}
	results := make([]MetricItem, 0, 16)

	for rows.Next() {
		var item MetricItem

		err := rows.Scan(&item.Process, &item.ProcessId, &item.Value, &item.Bucket, &item.MetricName, &item.MetricLabel)
		if err != nil {
			log.Printf("failed to get record: %v", err)
			continue
		}

		results = append(results, item)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"count":         len(results),
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiMetricsContainers(c *gin.Context) {
	nodeId := s.Param(c, "nodeId")
	nodeQuery := ""
	if nodeId != "" {
		nodeQuery = fmt.Sprintf(" AND metrics.node_id=%s", nodeId)
	}

	containerId := s.Param(c, "containerId")
	containerQuery := ""
	if containerId != "" {
		containerQuery = fmt.Sprintf(" AND metrics.container_id=%s", containerId)
	}

	cId := s.Param(c, "clusterId")
	query := s.ParseQuery(c)
	if s.IsValidParams(cId, query, true, true) == false {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}

	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND metrics.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	truncateQuery := s.calculateGranularity(query.DateRange, query.Timezone, query.Granularity)

	q := fmt.Sprintf(`
SELECT containers.name as container, containers.id, ROUND(value, 2), bucket,
       metric_names.name, metric_labels.label FROM
    (SELECT metrics.container_id as container_id, avg(value) as value,
            metrics.name_id, metrics.label_id, %s
    FROM metrics
    WHERE ts >= '%s' AND ts < '%s'
      AND metrics.cluster_id=%s %s %s %s
    GROUP BY bucket, metrics.container_id, metrics.name_id, metrics.label_id)
        as metrics_bucket, metric_names, metric_labels, containers
WHERE
    metrics_bucket.container_id=containers.id AND
      metrics_bucket.name_id=metric_names.id AND
      metrics_bucket.label_id=metric_labels.id
ORDER BY bucket`, truncateQuery, query.DateRange[0], query.DateRange[1],
		cId, nodeQuery, containerQuery, metricNameQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(q))
	if err != nil {
		log.Printf("failed to get metric data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("unexpected error: %v", err))
		return
	}

	type MetricItem struct {
		Container   string  `json:"container"`
		ContainerId uint    `json:"container_id"`
		Value       float64 `json:"value"`
		Bucket      string  `json:"bucket"`
		MetricName  string  `json:"metric_name"`
		MetricLabel string  `json:"metric_label"`
	}
	results := make([]MetricItem, 0, 16)

	for rows.Next() {
		var item MetricItem

		err := rows.Scan(&item.Container, &item.ContainerId,
			&item.Value, &item.Bucket, &item.MetricName, &item.MetricLabel)
		if err != nil {
			log.Printf("failed to get record: %v", err)
			continue
		}

		results = append(results, item)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"count":         len(results),
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) ApiMetricsPods(c *gin.Context) {
	namespaceId := s.Param(c, "namespaceId")
	namespaceQuery := ""
	if namespaceId != "" {
		namespaceQuery = fmt.Sprintf(" AND k8s_namespaces.id=%s", namespaceId)
	}

	podId := s.Param(c, "podId")
	podQuery := ""
	if podId != "" {
		podQuery = fmt.Sprintf(" AND k8s_pods.id=%s", podId)
	}

	cId := s.Param(c, "clusterId")
	query := s.ParseQuery(c)
	if s.IsValidParams(cId, query, true, true) == false {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}

	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND metrics.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	truncateQuery := s.calculateGranularity(query.DateRange, query.Timezone, query.Granularity)

	q := fmt.Sprintf(`
SELECT k8s_pods.name as pod, k8s_namespaces.name as namespace,
       ROUND(SUM(value), 2) as value, bucket, metric_names.name
FROM
    (SELECT metrics.container_id as container_id, avg(value) as value,
            metrics.name_id, metrics.label_id, %s
    FROM metrics
    WHERE ts >= '%s' AND ts < '%s'
      AND metrics.cluster_id=%s %s
    GROUP BY bucket, metrics.container_id, metrics.name_id, metrics.label_id)
        as metrics_bucket, metric_names, containers, k8s_pods, k8s_containers, k8s_namespaces
WHERE
    metrics_bucket.container_id=containers.id
    AND metrics_bucket.name_id=metric_names.id
    AND containers.container_id=k8s_containers.container_id
    AND k8s_containers.k8s_pod_id=k8s_pods.id
    AND k8s_pods.k8s_namespace_id=k8s_namespaces.id %s %s
GROUP BY bucket, pod, namespace, metric_names.name
ORDER BY bucket`, truncateQuery, query.DateRange[0], query.DateRange[1],
		cId, metricNameQuery, namespaceQuery, podQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(q))

	if err != nil {
		log.Printf("failed to get metric data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("unexpected error: %v", err))
		return
	}

	type MetricItem struct {
		Pod        string  `json:"pod"`
		Namespace  string  `json:"namespace"`
		Value      float64 `json:"value"`
		Bucket     string  `json:"bucket"`
		MetricName string  `json:"metric_name"`
	}
	results := make([]MetricItem, 0, 16)

	for rows.Next() {
		var item MetricItem

		err := rows.Scan(&item.Pod, &item.Namespace, &item.Value, &item.Bucket, &item.MetricName)
		if err != nil {
			log.Printf("failed to get record: %v", err)
			continue
		}

		results = append(results, item)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"count":         len(results),
		"db_query_time": queryTime.String(),
	})
}

func (s *NexServer) calculateGranularity(dateRanges []string, timezone, granularity string) string {
	if dateRanges == nil || len(dateRanges) != 2 {
		return ""
	}

	bucket := ""
	for _, wantedBucket := range []string{"minute", "hour", "day", "month", "year"} {
		if granularity == wantedBucket {
			bucket = granularity
			break
		}
	}
	if bucket != "" {
		truncateQuery := fmt.Sprintf(`DATE_TRUNC('%s', ts AT TIME ZONE '%s') as bucket`, bucket, timezone)

		return truncateQuery
	}

	start, err := time.Parse(time.RFC3339, dateRanges[0])
	if err != nil {
		start, err = time.Parse("2006-01-02 15:04:05", dateRanges[0])
		if err != nil {
			return ""
		}
	}
	end, err := time.Parse(time.RFC3339, dateRanges[1])
	if err != nil {
		end, err = time.Parse("2006-01-02 15:04:05", dateRanges[1])
		if err != nil {
			return ""
		}
	}

	diff := end.Sub(start).Minutes()
	interval := int64(diff/60.0) * 5 / 5
	if interval == 0 {
		interval = 1
	}
	truncateQuery := ""

	if interval < 60 {
		truncateQuery = fmt.Sprintf(`
			DATE_TRUNC('hour', ts) +
			DATE_PART('minute', ts)::int / %d * INTERVAL '%d minute' as bucket`,
			interval, interval)
	} else if interval < 1440 {
		interval /= 60
		truncateQuery = fmt.Sprintf(`
			DATE_TRUNC('day', ts) +
			DATE_PART('hour', ts)::int / %d * INTERVAL '%d hour' as bucket`,
			interval, interval)
	} else {
		interval /= 1440
		truncateQuery = fmt.Sprintf(`
			DATE_TRUNC('month', ts) +
			DATE_PART('day', ts)::int / %d * INTERVAL '%d day' as bucket`,
			interval, interval)
	}

	return truncateQuery
}

func (s *NexServer) ApiIncidentBasic(c *gin.Context) {
	incidents := make([]*IncidentItem, 0, 16)

	for eventName := range s.incidentMap {
		incidents = append(incidents, s.incidentMap[eventName]...)
	}

	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].DetectedTs.Unix() >= incidents[j].DetectedTs.Unix()
	})

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    incidents,
	})
}

func (s *NexServer) ApiMetricsClusterSummary(c *gin.Context) {
	cId := s.Param(c, "clusterId")
	query := s.ParseQuery(c)
	if s.IsValidParams(cId, query, true, true) == false {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}

	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(query.MetricNames) != len(metricNameIds) {
		s.ApiResponseJson(c, 404, "bad", "invalid query parameters")
		return
	}
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND metrics.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}

	truncateQuery := s.calculateGranularity(query.DateRange, query.Timezone, query.Granularity)

	metricQuery := fmt.Sprintf(`
SELECT ROUND(value, 2) as value, bucket, metric_names.name 
FROM
    (SELECT avg(value) as value, metrics.name_id, %s
    FROM metrics
    WHERE ts >= '%s' AND ts < '%s' AND metrics.cluster_id=%s 
      AND metrics.process_id=0
      AND metrics.container_id=0 %s
    GROUP BY bucket, metrics.name_id)
        as metrics_bucket, metric_names
WHERE
    metrics_bucket.name_id=metric_names.id
ORDER BY bucket`, truncateQuery, query.DateRange[0], query.DateRange[1], cId, metricNameQuery)

	rows, err, queryTime := s.QueryRowsWithTime(s.db.Raw(metricQuery))

	if err != nil {
		log.Printf("failed to get metric data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("unexpected error: %v", err))
		return
	}

	type MetricItem struct {
		Value      float64 `json:"value"`
		Bucket     string  `json:"bucket"`
		MetricName string  `json:"metric_name"`
	}
	results := make([]MetricItem, 0, 16)

	for rows.Next() {
		var item MetricItem

		err := rows.Scan(&item.Value, &item.Bucket, &item.MetricName)
		if err != nil {
			log.Printf("failed to get record: %v", err)
			continue
		}

		results = append(results, item)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       "",
		"data":          results,
		"count":         len(results),
		"db_query_time": queryTime.String(),
	})
}
