package collector

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
)

// scrapeArchivedInfo 归档日志空间信息
func (e *Exporter) scrapeArchivedInfo(db *sql.DB, ch chan<- prometheus.Metric) error {
	fmt.Printf("starting scrapeArchivedInfo \n")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(e.config.QueryTimeout)*time.Second)
	defer cancel()
	var archiveStatus string
	if err := db.QueryRowContext(ctx, `select log_mode from v$database`).Scan(&archiveStatus); err != nil {
		return err
	}
	level.Debug(e.logger).Log(fmt.Printf("archiveStatus: %v \n", archiveStatus))
	if strings.ToUpper(archiveStatus) != "ARCHIVELOG" {
		return nil
	}

	archDest := e.findArchDest(db)
	level.Debug(e.logger).Log(fmt.Printf("archDest: %v \n", archDest))
	if archDest == "" {
		return nil
	}

	if strings.Contains(archDest, "+") {
		archDest := strings.Replace(archDest, "+", "", -1)
		if strings.Contains(archDest, "/") {
			archDest = strings.Split(archDest, "/")[0]
		}
		var diskgrouName, state string
		var totalMb, freeMb, usage float64
		sqlStr := fmt.Sprintf("select NAME, state ,total_MB, free_MB from v$asm_diskgroup where name='%s'", archDest)
		if err := db.QueryRow(sqlStr).Scan(&diskgrouName, &state, &totalMb, &freeMb); err != nil {
			level.Error(e.logger).Log(fmt.Printf("query from asm_disgroup error: %v \n", err))
			return nil
		}
		usage = 100 * (totalMb - freeMb) / totalMb
		level.Debug(e.logger).Log(fmt.Printf("scrape asm disk usage:%v by diskgrou name: %v \n", usage, diskgrouName))

		ch <- prometheus.MustNewConstMetric(NewDesc("archived_log_total", "ArchivedLogTotalSize", nil, prometheus.Labels{"diskgroup_name": diskgrouName}),
			prometheus.GaugeValue, totalMb*1024*1024) // 统一以bytes单位输出
		ch <- prometheus.MustNewConstMetric(NewDesc("archived_log_used", "ArchivedLogUsedSize", nil, prometheus.Labels{"diskgroup_name": diskgrouName}),
			prometheus.GaugeValue, (totalMb-freeMb)*1024*1024) // 统一以bytes单位输出
		ch <- prometheus.MustNewConstMetric(NewDesc("archived_log_usage_ratio", "ArchivedLogUsage", nil, prometheus.Labels{"diskgroup_name": diskgrouName}),
			prometheus.GaugeValue, usage)

	}
	//else {
	//	// 从本地磁盘获取
	//	diskInfo, _ := disk.Usage(archDest)
	//	if diskInfo == nil {
	//		return nil
	//	}
	//	var total, usage, usageRatio float64
	//	//var total, usage float64
	//	usageRatio = diskInfo.UsedPercent
	//	total = float64(diskInfo.Total)
	//	usage = float64(diskInfo.Used)
	//	ch <- prometheus.MustNewConstMetric(NewDesc("archived_log_total", "ArchivedLogTotalSize", nil),
	//		prometheus.GaugeValue, total)
	//	ch <- prometheus.MustNewConstMetric(NewDesc("archived_log_used", "ArchivedLogUsedSize", nil),
	//		prometheus.GaugeValue, usage)
	//	ch <- prometheus.MustNewConstMetric(NewDesc("archived_log_usage_ratio", "ArchivedLogUsage", nil),
	//		prometheus.GaugeValue, usageRatio)
	//}
	return nil
}

func (e *Exporter) findArchDest(db *sql.DB) string {
	var archiveStatus string
	if err := db.QueryRow(`SELECT destination from v$archive_dest where dest_id=1 and rownum=1`).Scan(&archiveStatus); err != nil {
		level.Error(e.logger).Log(fmt.Printf("findArchDest query destination err: %v \n", err))
		return ""
	}
	level.Debug(e.logger).Log(fmt.Printf("findArchDest archiveStatus: %s \n", archiveStatus))

	if archiveStatus == "USE_DB_RECOVERY_FILE_DEST" {
		if err := db.QueryRow(`select value from v$parameter where name = 'db_recovery_file_dest'`).Scan(&archiveStatus); err != nil {
			level.Error(e.logger).Log(fmt.Printf("findArchDest query db_recovery_file_dest err: %v \n", err))
			return ""
		}
	}
	return archiveStatus
}

func NewDesc(name string, help string, label []string, dimensions prometheus.Labels) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name),
		fmt.Sprintf("Gauge metric with %v", help), label, dimensions)
}
