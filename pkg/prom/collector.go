package prom

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/amoghe/distillog"
	"github.com/mjavier2k/solidfire-exporter/pkg/solidfire"
	"golang.org/x/sync/errgroup"

	"github.com/prometheus/client_golang/prometheus"
)

type volumeMetadata struct {
	VolumeId  string
	Name      string
	AccountId string
}

func (v *volumeMetadata) Values() []string {
	return []string{
		v.VolumeId,
		v.Name,
		v.AccountId,
	}
}

type SolidfireCollector struct {
	client             solidfire.Interface
	timeout            time.Duration
	volumeMetadataByID map[int]volumeMetadata
	nodesNamesByID     map[int]string
}
type CollectorOpts struct {
	Client  solidfire.Interface
	Timeout time.Duration
}

var (
	mu                    sync.Mutex
	MetricDescriptions    = NewMetricDescriptions("solidfire")
	possibleDriveStatuses = []string{"active", "available", "erasing", "failed", "removing"}
)

func sumHistogram(m map[float64]uint64) (r uint64) {
	r = 0
	for _, val := range m {
		r += val
	}
	return
}

func strCompare(str1 string, str2 string) int {
	if strings.Compare(strings.ToLower(str1), strings.ToLower(str2)) == 0 {
		return 1
	}
	return 0
}

func (c *SolidfireCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- MetricDescriptions.upDesc

	ch <- MetricDescriptions.VolumeActualIOPS
	ch <- MetricDescriptions.VolumeAverageIOPSizeBytes
	ch <- MetricDescriptions.VolumeBurstIOPSCredit
	ch <- MetricDescriptions.VolumeClientQueueDepth
	ch <- MetricDescriptions.VolumeLatencySeconds
	ch <- MetricDescriptions.VolumeNonZeroBlocks
	ch <- MetricDescriptions.VolumeReadBytesTotal
	ch <- MetricDescriptions.VolumeReadLatencySecondsTotal
	ch <- MetricDescriptions.VolumeReadOpsTotal
	ch <- MetricDescriptions.VolumeThrottle
	ch <- MetricDescriptions.VolumeUnalignedReadsTotal
	ch <- MetricDescriptions.VolumeUnalignedWritesTotal
	ch <- MetricDescriptions.VolumeSizeBytes
	ch <- MetricDescriptions.VolumeUtilization
	ch <- MetricDescriptions.VolumeWriteBytesTotal
	ch <- MetricDescriptions.VolumeWriteLatencyTotal
	ch <- MetricDescriptions.VolumeWriteOpsTotal
	ch <- MetricDescriptions.VolumeStatsZeroBlocks

	ch <- MetricDescriptions.ClusterActiveBlockSpaceBytes
	ch <- MetricDescriptions.ClusterActiveSessions
	ch <- MetricDescriptions.ClusterAverageIOPS
	ch <- MetricDescriptions.ClusterClusterRecentIOSizeBytes
	ch <- MetricDescriptions.ClusterCurrentIOPS
	ch <- MetricDescriptions.ClusterMaxIOPS
	ch <- MetricDescriptions.ClusterMaxOverProvisionableSpaceBytes
	ch <- MetricDescriptions.ClusterMaxProvisionedSpaceBytes
	ch <- MetricDescriptions.ClusterMaxUsedMetadataSpaceBytes
	ch <- MetricDescriptions.ClusterMaxUsedSpaceBytes
	ch <- MetricDescriptions.ClusterNonZeroBlocks
	ch <- MetricDescriptions.ClusterPeakActiveSessions
	ch <- MetricDescriptions.ClusterPeakIOPS
	ch <- MetricDescriptions.ClusterProvisionedSpaceBytes
	ch <- MetricDescriptions.ClusterSnapshotNonZeroBlocks
	ch <- MetricDescriptions.ClusterIOPSTotal
	ch <- MetricDescriptions.ClusterUniqueBlocks
	ch <- MetricDescriptions.ClusterUniqueBlocksUsedSpaceBytes
	ch <- MetricDescriptions.ClusterUsedMetadataSpaceBytes
	ch <- MetricDescriptions.ClusterUsedMetadataSpaceInSnapshotsBytes
	ch <- MetricDescriptions.ClusterUsedSpaceBytes
	ch <- MetricDescriptions.ClusterZeroBlocks
	ch <- MetricDescriptions.ClusterThinProvisioningFactor
	ch <- MetricDescriptions.ClusterDeDuplicationFactor
	ch <- MetricDescriptions.ClusterCompressionFactor
	ch <- MetricDescriptions.ClusterEfficiencyFactor

	ch <- MetricDescriptions.ClusterActiveFaults

	ch <- MetricDescriptions.NodeSamples
	ch <- MetricDescriptions.NodeCPUPercentage
	ch <- MetricDescriptions.NodeCPUSecondsTotal
	ch <- MetricDescriptions.NodeInterfaceInBytesTotal
	ch <- MetricDescriptions.NodeInterfaceOutBytesTotal
	ch <- MetricDescriptions.NodeInterfaceUtilizationPercentage
	ch <- MetricDescriptions.NodeReadLatencyTotal
	ch <- MetricDescriptions.NodeUsedMemoryBytes
	ch <- MetricDescriptions.NodeWriteLatencyTotal
	ch <- MetricDescriptions.NodeLoadHistogram

	ch <- MetricDescriptions.NodeInfo

	ch <- MetricDescriptions.ClusterActualIOPS
	ch <- MetricDescriptions.ClusterAverageIOBytes
	ch <- MetricDescriptions.ClusterClientQueueDepth
	ch <- MetricDescriptions.ClusterThroughputUtilization
	ch <- MetricDescriptions.ClusterLatencySeconds
	ch <- MetricDescriptions.ClusterNormalizedIOPS
	ch <- MetricDescriptions.ClusterReadBytesTotal
	ch <- MetricDescriptions.ClusterLastSampleReadBytes
	ch <- MetricDescriptions.ClusterReadLatencySeconds
	ch <- MetricDescriptions.ClusterReadLatencyTotal
	ch <- MetricDescriptions.ClusterReadOpsTotal
	ch <- MetricDescriptions.ClusterLastSampleReadOps
	ch <- MetricDescriptions.ClusterSamplePeriodSeconds
	ch <- MetricDescriptions.ClusterServices
	ch <- MetricDescriptions.ClusterExpectedServices
	ch <- MetricDescriptions.ClusterUnalignedReadsTotal
	ch <- MetricDescriptions.ClusterUnalignedWritesTotal
	ch <- MetricDescriptions.ClusterWriteBytesTotal
	ch <- MetricDescriptions.ClusterLastSampleWriteBytes
	ch <- MetricDescriptions.ClusterWriteLatency
	ch <- MetricDescriptions.ClusterWriteLatencyTotal
	ch <- MetricDescriptions.ClusterWriteOpsTotal
	ch <- MetricDescriptions.ClusterLastSampleWriteOps

	ch <- MetricDescriptions.ClusterBlockFullness
	ch <- MetricDescriptions.ClusterFullness
	ch <- MetricDescriptions.ClusterMaxMetadataOverProvisionFactor
	ch <- MetricDescriptions.ClusterMetadataFullness
	ch <- MetricDescriptions.ClusterSliceReserveUsedThresholdPercentage
	ch <- MetricDescriptions.ClusterStage2AwareThresholdPercentage
	ch <- MetricDescriptions.ClusterStage2BlockThresholdBytes
	ch <- MetricDescriptions.ClusterStage3BlockThresholdBytes
	ch <- MetricDescriptions.ClusterStage3BlockThresholdPercentage
	ch <- MetricDescriptions.ClusterStage3LowThresholdPercentage
	ch <- MetricDescriptions.ClusterStage4BlockThresholdBytes
	ch <- MetricDescriptions.ClusterStage4CriticalThreshold
	ch <- MetricDescriptions.ClusterStage5BlockThresholdBytes
	ch <- MetricDescriptions.ClusterTotalBytes
	ch <- MetricDescriptions.ClusterTotalMetadataBytes
	ch <- MetricDescriptions.ClusterUsedBytes
	ch <- MetricDescriptions.ClusterUsedMetadataBytes

	ch <- MetricDescriptions.DriveStatus
	ch <- MetricDescriptions.DriveCapacityBytes

	ch <- MetricDescriptions.NodeISCSISessions

	ch <- MetricDescriptions.VolumeCount
	ch <- MetricDescriptions.AccountCount
	ch <- MetricDescriptions.ClusterAdminCount
	ch <- MetricDescriptions.InitiatorCount
	ch <- MetricDescriptions.VolumeAccessGroupCount
	ch <- MetricDescriptions.VirtualVolumeTasks
	ch <- MetricDescriptions.BulkVolumeJobs
	ch <- MetricDescriptions.AsyncResultsActive
	ch <- MetricDescriptions.AsyncResults
	ch <- MetricDescriptions.MaxAsyncResultID
}

func (c *SolidfireCollector) collectVolumeMeta(ctx context.Context, ch chan<- prometheus.Metric) error {
	volumes, err := c.client.ListVolumes(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()

	volumeCntByStatus := map[string]int{}

	for _, vol := range volumes.Result.Volumes {
		metadata := volumeMetadata{
			Name:     vol.Name,
			VolumeId: strconv.Itoa(vol.VolumeID),
		}
		ownerId, ok := vol.Attributes["owner_id"]
		if ok {
			metadata.AccountId = ownerId
		}
		c.volumeMetadataByID[vol.VolumeID] = metadata
		volumeCntByStatus[vol.Status]++
	}

	for status, count := range volumeCntByStatus {
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeCount,
			prometheus.CounterValue,
			float64(count),
			status,
		)
	}
	return nil
}

func (c *SolidfireCollector) collectNodeMeta(ctx context.Context, ch chan<- prometheus.Metric) error {
	nodes, err := c.client.ListAllNodes(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for _, node := range nodes.Result.Nodes {
		c.nodesNamesByID[node.NodeID] = node.Name
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInfo,
			prometheus.GaugeValue,
			1,
			strconv.Itoa(node.NodeID),
			node.Name,
			node.ChassisName,
			strconv.Itoa(node.AssociatedFServiceID),
			strconv.Itoa(node.AssociatedMasterServiceID),
			node.PlatformInfo.ChassisType,
			node.PlatformInfo.CPUModel,
			node.PlatformInfo.NodeType,
			node.PlatformInfo.PlatformConfigVersion,
			node.Sip,
			node.Sipi,
			node.SoftwareVersion,
			node.UUID,
		)
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeTotalMemoryBytes,
			prometheus.GaugeValue,
			GigabytesToBytes(node.PlatformInfo.NodeMemoryGB),
			strconv.Itoa(node.NodeID),
			node.Name,
		)
	}
	return nil
}

func (c *SolidfireCollector) collectVolumeStats(ctx context.Context, ch chan<- prometheus.Metric) error {
	volumeStats, err := c.client.ListVolumeStats(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for _, vol := range volumeStats.Result.VolumeStats {
		metadata := c.volumeMetadataByID[vol.VolumeID]
		name := metadata.Name
		values := metadata.Values()

		if ok, _ := regexp.MatchString(`snapshot-clone-src-*|replica-vol-*`, name); ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeActualIOPS,
			prometheus.GaugeValue,
			vol.ActualIOPS,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeAverageIOPSizeBytes,
			prometheus.GaugeValue,
			vol.AverageIOPSize,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeBurstIOPSCredit,
			prometheus.GaugeValue,
			vol.BurstIOPSCredit,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeClientQueueDepth,
			prometheus.GaugeValue,
			vol.ClientQueueDepth,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeLatencySeconds,
			prometheus.GaugeValue,
			MicrosecondsToSeconds(vol.LatencyUSec),
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeNonZeroBlocks,
			prometheus.GaugeValue,
			vol.NonZeroBlocks,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeReadBytesTotal,
			prometheus.CounterValue,
			vol.ReadBytes,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeReadLatencySecondsTotal,
			prometheus.CounterValue,
			MicrosecondsToSeconds(vol.ReadLatencyUSecTotal),
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeReadOpsTotal,
			prometheus.CounterValue,
			vol.ReadOps,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeThrottle,
			prometheus.GaugeValue,
			vol.Throttle,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeUnalignedReadsTotal,
			prometheus.CounterValue,
			vol.UnalignedReads,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeUnalignedWritesTotal,
			prometheus.CounterValue,
			vol.UnalignedWrites,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeSizeBytes,
			prometheus.GaugeValue,
			vol.VolumeSize,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeUtilization,
			prometheus.GaugeValue,
			vol.VolumeUtilization,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeWriteBytesTotal,
			prometheus.CounterValue,
			vol.WriteBytes,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeWriteLatencyTotal,
			prometheus.CounterValue,
			MicrosecondsToSeconds(vol.WriteLatencyUSecTotal),
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeWriteOpsTotal,
			prometheus.CounterValue,
			vol.WriteOps,
			values...)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.VolumeStatsZeroBlocks,
			prometheus.GaugeValue,
			vol.ZeroBlocks,
			values...)
	}
	return nil
}

func (c *SolidfireCollector) collectClusterCapacity(ctx context.Context, ch chan<- prometheus.Metric) error {
	clusterCapacity, err := c.client.GetClusterCapacity(ctx)
	if err != nil {
		return err
	}
	cluster := clusterCapacity.Result.ClusterCapacity
	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterActiveBlockSpaceBytes,
		prometheus.GaugeValue,
		cluster.ActiveBlockSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterActiveSessions,
		prometheus.GaugeValue,
		cluster.ActiveSessions)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterAverageIOPS,
		prometheus.GaugeValue,
		cluster.AverageIOPS)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterClusterRecentIOSizeBytes,
		prometheus.GaugeValue,
		cluster.ClusterRecentIOSize)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterCurrentIOPS,
		prometheus.GaugeValue,
		cluster.CurrentIOPS)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMaxIOPS,
		prometheus.GaugeValue,
		cluster.MaxIOPS)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMaxOverProvisionableSpaceBytes,
		prometheus.GaugeValue,
		cluster.MaxOverProvisionableSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMaxProvisionedSpaceBytes,
		prometheus.GaugeValue,
		cluster.MaxProvisionedSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMaxUsedMetadataSpaceBytes,
		prometheus.GaugeValue,
		cluster.MaxUsedMetadataSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMaxUsedSpaceBytes,
		prometheus.GaugeValue,
		cluster.MaxUsedSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterNonZeroBlocks,
		prometheus.GaugeValue,
		cluster.NonZeroBlocks)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterPeakActiveSessions,
		prometheus.GaugeValue,
		cluster.PeakActiveSessions)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterPeakIOPS,
		prometheus.GaugeValue,
		cluster.PeakIOPS)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterProvisionedSpaceBytes,
		prometheus.GaugeValue,
		cluster.ProvisionedSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterSnapshotNonZeroBlocks,
		prometheus.GaugeValue,
		cluster.SnapshotNonZeroBlocks)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterIOPSTotal,
		prometheus.CounterValue,
		cluster.TotalOps)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUniqueBlocks,
		prometheus.GaugeValue,
		cluster.UniqueBlocks)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUniqueBlocksUsedSpaceBytes,
		prometheus.GaugeValue,
		cluster.UniqueBlocksUsedSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUsedMetadataSpaceBytes,
		prometheus.GaugeValue,
		cluster.UsedMetadataSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUsedMetadataSpaceInSnapshotsBytes,
		prometheus.GaugeValue,
		cluster.UsedMetadataSpaceInSnapshots)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUsedSpaceBytes,
		prometheus.GaugeValue,
		cluster.UsedSpace)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterZeroBlocks,
		prometheus.GaugeValue,
		cluster.ZeroBlocks)

	clusterThinProvisioningFactor := (cluster.NonZeroBlocks + cluster.ZeroBlocks) / cluster.NonZeroBlocks
	if cluster.NonZeroBlocks == 0 {
		clusterThinProvisioningFactor = 1
	}

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterThinProvisioningFactor,
		prometheus.GaugeValue,
		clusterThinProvisioningFactor)

	clusterDeDuplicationFactor := (cluster.NonZeroBlocks + cluster.SnapshotNonZeroBlocks) / cluster.UniqueBlocks
	if cluster.UniqueBlocks == 0 {
		clusterDeDuplicationFactor = 1
	}

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterDeDuplicationFactor,
		prometheus.GaugeValue,
		clusterDeDuplicationFactor)

	clusterCompressionFactor := (cluster.UniqueBlocks * 4096) / (cluster.UniqueBlocksUsedSpace * 0.93)
	if cluster.UniqueBlocksUsedSpace == 0 {
		clusterCompressionFactor = 1
	}

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterCompressionFactor,
		prometheus.GaugeValue,
		clusterCompressionFactor)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterEfficiencyFactor,
		prometheus.GaugeValue,
		clusterThinProvisioningFactor*clusterDeDuplicationFactor*clusterCompressionFactor)
	return nil
}

func (c *SolidfireCollector) collectClusterFaults(ctx context.Context, ch chan<- prometheus.Metric) error {
	ClusterActiveFaults, err := c.client.ListClusterFaults(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for _, f := range ClusterActiveFaults.Result.Faults {
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.ClusterActiveFaults,
			prometheus.GaugeValue,
			1,
			strconv.Itoa(f.NodeID),
			c.nodesNamesByID[f.NodeID],
			f.Code,
			f.Severity,
			f.Type,
			fmt.Sprintf("%f", f.ServiceID),
			strconv.FormatBool(f.Resolved),
			fmt.Sprintf("%f", f.NodeHardwareFaultID),
			fmt.Sprintf("%f", f.DriveID),
			f.Details,
		)
	}
	return nil
}

func (c *SolidfireCollector) collectClusterNodeStats(ctx context.Context, ch chan<- prometheus.Metric) error {
	ClusterNodeStats, err := c.client.ListNodeStats(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for _, stats := range ClusterNodeStats.Result.NodeStats.Nodes {
		SsLoadHistogram := map[float64]uint64{
			0:   stats.SsLoadHistogram.Bucket0,
			19:  stats.SsLoadHistogram.Bucket1To19,
			39:  stats.SsLoadHistogram.Bucket20To39,
			59:  stats.SsLoadHistogram.Bucket40To59,
			79:  stats.SsLoadHistogram.Bucket60To79,
			100: stats.SsLoadHistogram.Bucket80To100,
		}

		ch <- prometheus.MustNewConstHistogram(
			MetricDescriptions.NodeLoadHistogram,
			stats.Count,
			float64(sumHistogram(SsLoadHistogram)),
			SsLoadHistogram,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceInBytesTotal,
			prometheus.CounterValue,
			stats.CBytesIn,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"cluster",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceOutBytesTotal,
			prometheus.CounterValue,
			stats.CBytesOut,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"cluster",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeSamples,
			prometheus.GaugeValue,
			float64(stats.Count),
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeCPUPercentage,
			prometheus.GaugeValue,
			stats.CPU,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeCPUSecondsTotal,
			prometheus.CounterValue,
			stats.CPUTotal,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceInBytesTotal,
			prometheus.CounterValue,
			stats.MBytesIn,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"management",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceOutBytesTotal,
			prometheus.CounterValue,
			stats.MBytesOut,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"management",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceUtilizationPercentage,
			prometheus.GaugeValue,
			stats.NetworkUtilizationCluster,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"cluster",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceUtilizationPercentage,
			prometheus.GaugeValue,
			stats.NetworkUtilizationStorage,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"storage",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeReadLatencyTotal,
			prometheus.CounterValue,
			MicrosecondsToSeconds(stats.ReadLatencyUSecTotal),
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceInBytesTotal,
			prometheus.CounterValue,
			stats.SBytesIn,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"storage",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeInterfaceOutBytesTotal,
			prometheus.CounterValue,
			stats.SBytesOut,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
			"storage",
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeUsedMemoryBytes,
			prometheus.GaugeValue,
			stats.UsedMemory,
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
		)

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeWriteLatencyTotal,
			prometheus.CounterValue,
			MicrosecondsToSeconds(stats.WriteLatencyUSecTotal),
			strconv.Itoa(stats.NodeID),
			c.nodesNamesByID[stats.NodeID],
		)

	}
	return nil
}

func (c *SolidfireCollector) collectVolumeQosHistograms(ctx context.Context, ch chan<- prometheus.Metric) error {
	VolumeQoSHistograms, err := c.client.ListVolumeQoSHistograms(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for _, h := range VolumeQoSHistograms.Result.QosHistograms {
		metadata := c.volumeMetadataByID[h.VolumeID]
		name := metadata.Name
		values := metadata.Values()
		if ok, _ := regexp.MatchString(`snapshot-clone-src-*|replica-vol-*`, name); ok {
			continue
		}
		// Below Min IOPS Percentage
		BelowMinIopsPercentages := map[float64]uint64{
			19:  h.Histograms.BelowMinIopsPercentages.Bucket1To19,
			39:  h.Histograms.BelowMinIopsPercentages.Bucket20To39,
			59:  h.Histograms.BelowMinIopsPercentages.Bucket40To59,
			79:  h.Histograms.BelowMinIopsPercentages.Bucket60To79,
			100: h.Histograms.BelowMinIopsPercentages.Bucket80To100,
		}

		ch <- prometheus.MustNewConstHistogram(
			MetricDescriptions.VolumeQoSBelowMinIopsPercentagesHistogram,
			0,
			float64(sumHistogram(BelowMinIopsPercentages)),
			BelowMinIopsPercentages,
			values...)

		MinToMaxIopsPercentages := map[float64]uint64{
			19:          h.Histograms.MinToMaxIopsPercentages.Bucket1To19,
			39:          h.Histograms.MinToMaxIopsPercentages.Bucket20To39,
			59:          h.Histograms.MinToMaxIopsPercentages.Bucket40To59,
			79:          h.Histograms.MinToMaxIopsPercentages.Bucket60To79,
			100:         h.Histograms.MinToMaxIopsPercentages.Bucket80To100,
			math.Inf(1): h.Histograms.MinToMaxIopsPercentages.Bucket101Plus,
		}

		ch <- prometheus.MustNewConstHistogram(
			MetricDescriptions.VolumeQoSMinToMaxIopsPercentagesHistogram,
			0,
			float64(sumHistogram(MinToMaxIopsPercentages)),
			MinToMaxIopsPercentages,
			values...)

		ReadBlockSizes := map[float64]uint64{
			8191:        h.Histograms.ReadBlockSizes.Bucket4096To8191,
			16383:       h.Histograms.ReadBlockSizes.Bucket8192To16383,
			32767:       h.Histograms.ReadBlockSizes.Bucket16384To32767,
			65535:       h.Histograms.ReadBlockSizes.Bucket32768To65535,
			131071:      h.Histograms.ReadBlockSizes.Bucket65536To131071,
			math.Inf(1): h.Histograms.ReadBlockSizes.Bucket131072Plus,
		}

		ch <- prometheus.MustNewConstHistogram(
			MetricDescriptions.VolumeQoSReadBlockSizesHistogram,
			0,
			float64(sumHistogram(ReadBlockSizes)),
			ReadBlockSizes,
			values...)

		TargetUtilizationPercentages := map[float64]uint64{
			0:           h.Histograms.TargetUtilizationPercentages.Bucket0,
			19:          h.Histograms.TargetUtilizationPercentages.Bucket1To19,
			39:          h.Histograms.TargetUtilizationPercentages.Bucket20To39,
			59:          h.Histograms.TargetUtilizationPercentages.Bucket40To59,
			79:          h.Histograms.TargetUtilizationPercentages.Bucket60To79,
			100:         h.Histograms.TargetUtilizationPercentages.Bucket80To100,
			math.Inf(1): h.Histograms.TargetUtilizationPercentages.Bucket101Plus,
		}

		ch <- prometheus.MustNewConstHistogram(
			MetricDescriptions.VolumeQoSTargetUtilizationPercentagesHistogram,
			0,
			float64(sumHistogram(TargetUtilizationPercentages)),
			TargetUtilizationPercentages,
			values...,
		)

		ThrottlePercentages := map[float64]uint64{
			0:   h.Histograms.ThrottlePercentages.Bucket0,
			19:  h.Histograms.ThrottlePercentages.Bucket1To19,
			39:  h.Histograms.ThrottlePercentages.Bucket20To39,
			59:  h.Histograms.ThrottlePercentages.Bucket40To59,
			79:  h.Histograms.ThrottlePercentages.Bucket60To79,
			100: h.Histograms.ThrottlePercentages.Bucket80To100,
		}

		ch <- prometheus.MustNewConstHistogram(
			MetricDescriptions.VolumeQoSThrottlePercentagesHistogram,
			0,
			float64(sumHistogram(ThrottlePercentages)),
			ThrottlePercentages,
			values...,
		)

		WriteBlockSizes := map[float64]uint64{
			8191:        h.Histograms.WriteBlockSizes.Bucket4096To8191,
			16383:       h.Histograms.WriteBlockSizes.Bucket8192To16383,
			32767:       h.Histograms.WriteBlockSizes.Bucket16384To32767,
			65535:       h.Histograms.WriteBlockSizes.Bucket32768To65535,
			131071:      h.Histograms.WriteBlockSizes.Bucket65536To131071,
			math.Inf(1): h.Histograms.WriteBlockSizes.Bucket131072Plus,
		}

		ch <- prometheus.MustNewConstHistogram(
			MetricDescriptions.VolumeQoSWriteBlockSizesHistogram,
			0,
			float64(sumHistogram(WriteBlockSizes)),
			WriteBlockSizes,
			values...,
		)
	}
	return nil
}

func (c *SolidfireCollector) collectClusterStats(ctx context.Context, ch chan<- prometheus.Metric) error {
	clusterStats, err := c.client.GetClusterStats(ctx)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterActualIOPS,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.ActualIOPS,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterAverageIOBytes,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.AverageIOPSize,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterClientQueueDepth,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.ClientQueueDepth,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterThroughputUtilization,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.ClusterUtilization,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterLatencySeconds,
		prometheus.GaugeValue,
		MicrosecondsToSeconds(clusterStats.Result.ClusterStats.LatencyUSec),
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterNormalizedIOPS,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.NormalizedIOPS,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterReadBytesTotal,
		prometheus.CounterValue,
		clusterStats.Result.ClusterStats.ReadBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterLastSampleReadBytes,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.ReadBytesLastSample,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterReadLatencySeconds,
		prometheus.GaugeValue,
		MicrosecondsToSeconds(clusterStats.Result.ClusterStats.ReadLatencyUSec),
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterReadLatencyTotal,
		prometheus.CounterValue,
		MicrosecondsToSeconds(clusterStats.Result.ClusterStats.ReadLatencyUSecTotal),
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterReadOpsTotal,
		prometheus.CounterValue,
		clusterStats.Result.ClusterStats.ReadOps,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterLastSampleReadOps,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.ReadOpsLastSample,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterSamplePeriodSeconds,
		prometheus.GaugeValue,
		MillisecondsToSeconds(clusterStats.Result.ClusterStats.SamplePeriodMsec),
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterServices,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.ServicesCount,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterExpectedServices,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.ServicesTotal,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUnalignedReadsTotal,
		prometheus.CounterValue,
		clusterStats.Result.ClusterStats.UnalignedReads,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUnalignedWritesTotal,
		prometheus.CounterValue,
		clusterStats.Result.ClusterStats.UnalignedWrites,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterWriteBytesTotal,
		prometheus.CounterValue,
		clusterStats.Result.ClusterStats.WriteBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterLastSampleWriteBytes,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.WriteBytesLastSample,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterWriteLatency,
		prometheus.GaugeValue,
		MicrosecondsToSeconds(clusterStats.Result.ClusterStats.WriteLatencyUSec),
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterWriteLatencyTotal,
		prometheus.CounterValue,
		MicrosecondsToSeconds(clusterStats.Result.ClusterStats.WriteLatencyUSecTotal),
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterWriteOpsTotal,
		prometheus.CounterValue,
		clusterStats.Result.ClusterStats.WriteOps,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterLastSampleWriteOps,
		prometheus.GaugeValue,
		clusterStats.Result.ClusterStats.WriteOpsLastSample,
	)
	return nil
}

func (c *SolidfireCollector) collectClusterFullThreshold(ctx context.Context, ch chan<- prometheus.Metric) error {
	clusterFullThreshold, err := c.client.GetClusterFullThreshold(ctx)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterBlockFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.BlockFullness, "stage1Happy")),
		"stage1Happy",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterBlockFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.BlockFullness, "stage2Aware")),
		"stage2Aware",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterBlockFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.BlockFullness, "stage3Low")),
		"stage3Low",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterBlockFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.BlockFullness, "stage4Critical")),
		"stage4Critical",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterBlockFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.BlockFullness, "stage5CompletelyConsumed")),
		"stage5CompletelyConsumed",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.Fullness, "blockFullness")),
		"blockFullness",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.Fullness, "metadataFullness")),
		"metadataFullness",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMaxMetadataOverProvisionFactor,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.MaxMetadataOverProvisionFactor,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMetadataFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.MetadataFullness, "stage1Happy")),
		"stage1Happy",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMetadataFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.MetadataFullness, "stage2Aware")),
		"stage2Aware",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMetadataFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.MetadataFullness, "stage3Low")),
		"stage3Low",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMetadataFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.MetadataFullness, "stage4Critical")),
		"stage4Critical",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterMetadataFullness,
		prometheus.GaugeValue,
		float64(strCompare(clusterFullThreshold.Result.MetadataFullness, "stage5CompletelyConsumed")),
		"stage5CompletelyConsumed",
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterSliceReserveUsedThresholdPercentage,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.SliceReserveUsedThresholdPct,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage2AwareThresholdPercentage,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage2AwareThreshold,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage2BlockThresholdBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage2BlockThresholdBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage3BlockThresholdBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage3BlockThresholdBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage3BlockThresholdPercentage,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage3BlockThresholdPercent,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage3LowThresholdPercentage,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage3LowThreshold,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage4BlockThresholdBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage4BlockThresholdBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage4CriticalThreshold,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage4CriticalThreshold,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterStage5BlockThresholdBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.Stage5BlockThresholdBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterTotalBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.SumTotalClusterBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterTotalMetadataBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.SumTotalMetadataClusterBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUsedBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.SumUsedClusterBytes,
	)

	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.ClusterUsedMetadataBytes,
		prometheus.GaugeValue,
		clusterFullThreshold.Result.SumUsedMetadataClusterBytes,
	)
	return nil
}

func (c *SolidfireCollector) collectDriveDetails(ctx context.Context, ch chan<- prometheus.Metric) error {
	ListDrives, err := c.client.ListDrives(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for _, d := range ListDrives.Result.Drives {
		for _, ds := range possibleDriveStatuses {
			var driveStatusValue float64 = 0
			if ds == d.Status {
				driveStatusValue = 1
			}
			ch <- prometheus.MustNewConstMetric(
				MetricDescriptions.DriveStatus,
				prometheus.GaugeValue,
				driveStatusValue,
				strconv.Itoa(d.NodeID),
				c.nodesNamesByID[d.NodeID],
				strconv.Itoa(d.DriveID),
				d.Serial,
				strconv.Itoa(d.Slot),
				ds,
				d.Type,
			)
		}

		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.DriveCapacityBytes,
			prometheus.GaugeValue,
			d.Capacity,
			strconv.Itoa(d.NodeID),
			c.nodesNamesByID[d.NodeID],
			strconv.Itoa(d.DriveID),
			d.Serial,
			strconv.Itoa(d.Slot),
			d.Type,
		)
	}
	return nil
}

func (c *SolidfireCollector) collectISCSISessions(ctx context.Context, ch chan<- prometheus.Metric) error {
	ListISCSISessions, err := c.client.ListISCSISessions(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	sessions := make(map[int]float64)

	for _, session := range ListISCSISessions.Result.Sessions {
		sessions[session.NodeID]++
	}

	for node, val := range sessions {
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.NodeISCSISessions,
			prometheus.GaugeValue,
			val,
			strconv.Itoa(node),
			c.nodesNamesByID[node],
		)
	}
	return nil
}

func (c *SolidfireCollector) collectAccounts(ctx context.Context, ch chan<- prometheus.Metric) error {
	accounts, err := c.client.ListAccounts(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.AccountCount,
		prometheus.CounterValue,
		float64(len(accounts.Result.Accounts)),
	)
	return nil
}

func (c *SolidfireCollector) collectInitiators(ctx context.Context, ch chan<- prometheus.Metric) error {
	initiators, err := c.client.ListInitiators(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.InitiatorCount,
		prometheus.CounterValue,
		float64(len(initiators.Result.Initiators)),
	)
	return nil
}

func (c *SolidfireCollector) collectVolumeAccessGroups(ctx context.Context, ch chan<- prometheus.Metric) error {
	volumeAccessGroups, err := c.client.ListVolumeAccessGroups(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.VolumeAccessGroupCount,
		prometheus.CounterValue,
		float64(len(volumeAccessGroups.Result.VolumeAccessGroups)),
	)
	return nil
}

func (c *SolidfireCollector) collectVirtualVolumeTasks(ctx context.Context, ch chan<- prometheus.Metric) error {
	vvt, err := c.client.ListVirtualVolumeTasks(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.VirtualVolumeTasks,
		prometheus.CounterValue,
		float64(len(vvt.Result.Tasks)),
	)
	return nil
}

func (c *SolidfireCollector) collectBulkVolumeJobs(ctx context.Context, ch chan<- prometheus.Metric) error {
	btj, err := c.client.ListBulkVolumeJobs(ctx)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.BulkVolumeJobs,
		prometheus.CounterValue,
		float64(len(btj.Result.BulkVolumeJobs)),
	)
	return nil
}

func (c *SolidfireCollector) collectAsyncResults(ctx context.Context, ch chan<- prometheus.Metric) error {
	ar, err := c.client.ListAsyncResults(ctx)
	if err != nil {
		return err
	}
	maxAsyncResultID := 0
	activeAsyncResults := make(map[string]int64)
	allAsyncResults := make(map[string]int64)
	for _, v := range ar.Result.AsyncHandles {
		allAsyncResults[v.ResultType]++
		if !v.Completed && !v.Success {
			activeAsyncResults[v.ResultType]++
		}
		if v.AsyncResultID > int64(maxAsyncResultID) {
			maxAsyncResultID = int(v.AsyncResultID)
		}
	}

	types := []string{"DriveAdd", "BulkVolume", "Clone", "DriveRemoval", "RtfiPendingNode"}
	for _, t := range types {
		if _, ok := activeAsyncResults[t]; !ok {
			activeAsyncResults[t] = 0
		}
		if _, ok := allAsyncResults[t]; !ok {
			allAsyncResults[t] = 0
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for k, v := range activeAsyncResults {
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.AsyncResultsActive,
			prometheus.GaugeValue,
			float64(v),
			k,
		)
	}
	for k, v := range allAsyncResults {
		ch <- prometheus.MustNewConstMetric(
			MetricDescriptions.AsyncResults,
			prometheus.GaugeValue,
			float64(v),
			k,
		)
	}
	ch <- prometheus.MustNewConstMetric(
		MetricDescriptions.MaxAsyncResultID,
		prometheus.GaugeValue,
		float64(maxAsyncResultID),
	)
	return nil
}

func (c *SolidfireCollector) Collect(ch chan<- prometheus.Metric) {
	var up float64 = 0
	defer func() { ch <- prometheus.MustNewConstMetric(MetricDescriptions.upDesc, prometheus.GaugeValue, up) }()
	timeout := c.timeout
	parentCtx, _ := context.WithTimeout(context.Background(), timeout)

	metadataGroup, ctx := errgroup.WithContext(parentCtx)
	metadataGroup.Go(func() error {
		return c.collectVolumeMeta(ctx, ch)
	})
	metadataGroup.Go(func() error {
		return c.collectNodeMeta(ctx, ch)
	})
	if err := metadataGroup.Wait(); err != nil {
		log.Errorln(err)
		return
	}
	metricsGroup, ctx := errgroup.WithContext(parentCtx)
	metricsGroup.Go(func() error {
		return c.collectVolumeStats(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectClusterCapacity(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectClusterFaults(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectClusterNodeStats(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectVolumeQosHistograms(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectClusterStats(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectClusterFullThreshold(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectDriveDetails(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectISCSISessions(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectAccounts(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectInitiators(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectVolumeAccessGroups(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectVirtualVolumeTasks(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectBulkVolumeJobs(ctx, ch)
	})
	metricsGroup.Go(func() error {
		return c.collectAsyncResults(ctx, ch)
	})
	if err := metricsGroup.Wait(); err != nil {
		log.Errorln(err)
		return
	}
	up = 1
	return
}

func NewCollector(opts *CollectorOpts) (*SolidfireCollector, error) {
	var err error
	if opts == nil {
		opts = &CollectorOpts{}
	}
	if opts.Client == nil {
		opts.Client, err = solidfire.NewSolidfireClient()
		if err != nil {
			return nil, err
		}
	}
	return &SolidfireCollector{
		volumeMetadataByID: make(map[int]volumeMetadata),
		nodesNamesByID:     make(map[int]string),
		client:             opts.Client,
		timeout:            opts.Timeout,
	}, nil
}

func GigabytesToBytes(gb float64) float64 {
	return gb * 1e+9
}

func MicrosecondsToSeconds(microSeconds float64) float64 {
	return microSeconds * 1e-6
}

func MillisecondsToSeconds(milliseconds float64) float64 {
	return milliseconds * 1e-3
}
