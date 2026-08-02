package o11y

// The INFRA reads — hosts, processes, and the Kubernetes fleet (pods, PVCs,
// nodes, namespaces, clusters, deployments, daemonsets, statefulsets, jobs),
// plus the infra_monitoring rollups and the setup checks — as TYPED ops.
//
// These forty-four routes reached traffic only through the delegation
// wildcard, and a route behind a wildcard is in no document: no SDK method, no
// CLI command, no agent tool, no reference page. A customer could not reach
// their own infrastructure telemetry from anything but a hand-written HTTP
// call. Typing them is what puts the operations in the document and therefore
// in every projection built from it.
//
// THE WIRE DOES NOT MOVE, the same way telemetry.go's five ops do not move it:
// these ops do not re-implement the reads. Each hands the call to the SAME
// runtime handler the wildcard delegates to (see infraRelay), so identity
// resolution, the org gate, the ViewAccess role check and the success envelope
// are all still the runtime's, executed in the order they always were. Every
// route here was mounted ViewAccess on mux (routes_hosts.go … routes_jobs.go,
// o11yapiserver/inframonitoring.go) and still is: the gate runs where it
// always ran, one layer in. What is new is the TYPE — the In a caller may send
// and the Out they get back — and the prose that goes with it.
//
// The request and response payloads are the RUNTIME'S OWN types
// (pkg/query-service/model, pkg/query-service/model/v3,
// pkg/types/inframonitoringtypes), not mirrors of them: the field list that
// decodes a request here is the field list the handler decodes one layer in,
// so the two cannot drift, and a field the runtime grows flows through without
// this file changing. Only the {status, data} envelopes and the GET
// query-parameter shapes are NAMED here, because the runtime spells those on
// the wire rather than in a type.

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/query-service/model"
	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	"github.com/hanzoai/o11y/pkg/types/inframonitoringtypes"
	"github.com/zap-proto/zip"
)

// mountInfra registers the forty-four typed infra ops on the native router:
// three per resource face (attribute_keys, attribute_values, list) for the
// eleven infra resources, the ten infra_monitoring rollups, and the setup
// checks.
func mountInfra(app *zip.App) {
	g := app.Group(o11yRoot)

	zip.Get(g, "/hosts/attribute_keys", hostAttributeKeys)
	zip.Get(g, "/hosts/attribute_values", hostAttributeValues)
	zip.Post(g, "/hosts/list", hostList)

	zip.Get(g, "/processes/attribute_keys", processAttributeKeys)
	zip.Get(g, "/processes/attribute_values", processAttributeValues)
	zip.Post(g, "/processes/list", processList)

	zip.Get(g, "/pods/attribute_keys", podAttributeKeys)
	zip.Get(g, "/pods/attribute_values", podAttributeValues)
	zip.Post(g, "/pods/list", podList)

	zip.Get(g, "/pvcs/attribute_keys", pvcAttributeKeys)
	zip.Get(g, "/pvcs/attribute_values", pvcAttributeValues)
	zip.Post(g, "/pvcs/list", pvcList)

	zip.Get(g, "/nodes/attribute_keys", nodeAttributeKeys)
	zip.Get(g, "/nodes/attribute_values", nodeAttributeValues)
	zip.Post(g, "/nodes/list", nodeList)

	zip.Get(g, "/namespaces/attribute_keys", namespaceAttributeKeys)
	zip.Get(g, "/namespaces/attribute_values", namespaceAttributeValues)
	zip.Post(g, "/namespaces/list", namespaceList)

	zip.Get(g, "/clusters/attribute_keys", clusterAttributeKeys)
	zip.Get(g, "/clusters/attribute_values", clusterAttributeValues)
	zip.Post(g, "/clusters/list", clusterList)

	zip.Get(g, "/deployments/attribute_keys", deploymentAttributeKeys)
	zip.Get(g, "/deployments/attribute_values", deploymentAttributeValues)
	zip.Post(g, "/deployments/list", deploymentList)

	zip.Get(g, "/daemonsets/attribute_keys", daemonSetAttributeKeys)
	zip.Get(g, "/daemonsets/attribute_values", daemonSetAttributeValues)
	zip.Post(g, "/daemonsets/list", daemonSetList)

	zip.Get(g, "/statefulsets/attribute_keys", statefulSetAttributeKeys)
	zip.Get(g, "/statefulsets/attribute_values", statefulSetAttributeValues)
	zip.Post(g, "/statefulsets/list", statefulSetList)

	zip.Get(g, "/jobs/attribute_keys", jobAttributeKeys)
	zip.Get(g, "/jobs/attribute_values", jobAttributeValues)
	zip.Post(g, "/jobs/list", jobList)

	zip.Post(g, "/infra_monitoring/hosts", infraHosts)
	zip.Post(g, "/infra_monitoring/pods", infraPods)
	zip.Post(g, "/infra_monitoring/nodes", infraNodes)
	zip.Post(g, "/infra_monitoring/namespaces", infraNamespaces)
	zip.Post(g, "/infra_monitoring/clusters", infraClusters)
	zip.Post(g, "/infra_monitoring/pvcs", infraVolumes)
	zip.Post(g, "/infra_monitoring/deployments", infraDeployments)
	zip.Post(g, "/infra_monitoring/statefulsets", infraStatefulSets)
	zip.Post(g, "/infra_monitoring/jobs", infraJobs)
	zip.Post(g, "/infra_monitoring/daemonsets", infraDaemonSets)
	zip.Get(g, "/infra_monitoring/checks", infraChecks)
}

// ── hosts ─────────────────────────────────────────────────────────────────────

// hostAttributeKeys lists the metric attribute keys hosts report, for building
// host filters — each with its data type and whether it is a materialized
// column.
func hostAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "hosts", in)
}

// hostAttributeValues lists the values one host attribute key has taken, for
// building host filters — string, number and bool values in their own lists.
func hostAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "hosts", in)
}

// hostList lists monitored hosts over a time range, each with its CPU,
// memory, I/O wait and 15-minute load, whether it is actively reporting, its
// OS and its attributes; filterable, groupable and paginated. The answer also
// says whether any host metrics were received at all and which clusters and
// nodes sent them, so an empty page is distinguishable from a fleet that never
// reported.
func hostList(ctx context.Context, in *model.HostListRequest) (*O11yHostListOut, error) {
	out := new(O11yHostListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/hosts/list", nil, in, out)
}

// ── processes ─────────────────────────────────────────────────────────────────

// processAttributeKeys lists the metric attribute keys processes report, for
// building process filters.
func processAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "processes", in)
}

// processAttributeValues lists the values one process attribute key has taken,
// for building process filters.
func processAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "processes", in)
}

// processList lists monitored processes over a time range, each with its name,
// PID, command line and CPU and memory usage; filterable, groupable and
// paginated.
func processList(ctx context.Context, in *model.ProcessListRequest) (*O11yProcessListOut, error) {
	out := new(O11yProcessListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/processes/list", nil, in, out)
}

// ── pods ──────────────────────────────────────────────────────────────────────

// podAttributeKeys lists the metric attribute keys Kubernetes pods report, for
// building pod filters.
func podAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "pods", in)
}

// podAttributeValues lists the values one pod attribute key has taken, for
// building pod filters.
func podAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "pods", in)
}

// podList lists Kubernetes pods over a time range, each with its CPU and
// memory usage against request and limit, restart count, phase counts and
// attributes; filterable, groupable and paginated.
func podList(ctx context.Context, in *model.PodListRequest) (*O11yPodListOut, error) {
	out := new(O11yPodListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/pods/list", nil, in, out)
}

// ── pvcs ──────────────────────────────────────────────────────────────────────

// pvcAttributeKeys lists the metric attribute keys persistent volume claims
// report, for building volume filters.
func pvcAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "pvcs", in)
}

// pvcAttributeValues lists the values one persistent-volume-claim attribute
// key has taken, for building volume filters.
func pvcAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "pvcs", in)
}

// pvcList lists Kubernetes persistent volume claims over a time range, each
// with its available, capacity and used bytes, inode counts and attributes;
// filterable, groupable and paginated.
func pvcList(ctx context.Context, in *model.VolumeListRequest) (*O11yPvcListOut, error) {
	out := new(O11yPvcListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/pvcs/list", nil, in, out)
}

// ── nodes ─────────────────────────────────────────────────────────────────────

// nodeAttributeKeys lists the metric attribute keys Kubernetes nodes report,
// for building node filters.
func nodeAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "nodes", in)
}

// nodeAttributeValues lists the values one node attribute key has taken, for
// building node filters.
func nodeAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "nodes", in)
}

// nodeList lists Kubernetes nodes over a time range, each with its CPU and
// memory usage against allocatable capacity, readiness condition counts and
// attributes; filterable, groupable and paginated.
func nodeList(ctx context.Context, in *model.NodeListRequest) (*O11yNodeListOut, error) {
	out := new(O11yNodeListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/nodes/list", nil, in, out)
}

// ── namespaces ────────────────────────────────────────────────────────────────

// namespaceAttributeKeys lists the metric attribute keys Kubernetes namespaces
// report, for building namespace filters.
func namespaceAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "namespaces", in)
}

// namespaceAttributeValues lists the values one namespace attribute key has
// taken, for building namespace filters.
func namespaceAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "namespaces", in)
}

// namespaceList lists Kubernetes namespaces over a time range, each with the
// CPU and memory its pods used, their phase counts and its attributes;
// filterable, groupable and paginated.
func namespaceList(ctx context.Context, in *model.NamespaceListRequest) (*O11yNamespaceListOut, error) {
	out := new(O11yNamespaceListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/namespaces/list", nil, in, out)
}

// ── clusters ──────────────────────────────────────────────────────────────────

// clusterAttributeKeys lists the metric attribute keys Kubernetes clusters
// report, for building cluster filters.
func clusterAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "clusters", in)
}

// clusterAttributeValues lists the values one cluster attribute key has taken,
// for building cluster filters.
func clusterAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "clusters", in)
}

// clusterList lists Kubernetes clusters over a time range, each with its CPU
// and memory usage against allocatable capacity and its attributes;
// filterable, groupable and paginated.
func clusterList(ctx context.Context, in *model.ClusterListRequest) (*O11yClusterListOut, error) {
	out := new(O11yClusterListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/clusters/list", nil, in, out)
}

// ── deployments ───────────────────────────────────────────────────────────────

// deploymentAttributeKeys lists the metric attribute keys Kubernetes
// deployments report, for building deployment filters.
func deploymentAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "deployments", in)
}

// deploymentAttributeValues lists the values one deployment attribute key has
// taken, for building deployment filters.
func deploymentAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "deployments", in)
}

// deploymentList lists Kubernetes deployments over a time range, each with the
// CPU and memory its pods used against request and limit, desired and
// available replica counts, restarts and attributes; filterable, groupable and
// paginated.
func deploymentList(ctx context.Context, in *model.DeploymentListRequest) (*O11yDeploymentListOut, error) {
	out := new(O11yDeploymentListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/deployments/list", nil, in, out)
}

// ── daemonsets ────────────────────────────────────────────────────────────────

// daemonSetAttributeKeys lists the metric attribute keys Kubernetes daemonsets
// report, for building daemonset filters.
func daemonSetAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "daemonsets", in)
}

// daemonSetAttributeValues lists the values one daemonset attribute key has
// taken, for building daemonset filters.
func daemonSetAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "daemonsets", in)
}

// daemonSetList lists Kubernetes daemonsets over a time range, each with the
// CPU and memory its pods used against request and limit, desired and
// available node counts, restarts and attributes; filterable, groupable and
// paginated.
func daemonSetList(ctx context.Context, in *model.DaemonSetListRequest) (*O11yDaemonSetListOut, error) {
	out := new(O11yDaemonSetListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/daemonsets/list", nil, in, out)
}

// ── statefulsets ──────────────────────────────────────────────────────────────

// statefulSetAttributeKeys lists the metric attribute keys Kubernetes
// statefulsets report, for building statefulset filters.
func statefulSetAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "statefulsets", in)
}

// statefulSetAttributeValues lists the values one statefulset attribute key
// has taken, for building statefulset filters.
func statefulSetAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "statefulsets", in)
}

// statefulSetList lists Kubernetes statefulsets over a time range, each with
// the CPU and memory its pods used against request and limit, desired and
// available replica counts, restarts and attributes; filterable, groupable and
// paginated.
func statefulSetList(ctx context.Context, in *model.StatefulSetListRequest) (*O11yStatefulSetListOut, error) {
	out := new(O11yStatefulSetListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/statefulsets/list", nil, in, out)
}

// ── jobs ──────────────────────────────────────────────────────────────────────

// jobAttributeKeys lists the metric attribute keys Kubernetes jobs report, for
// building job filters.
func jobAttributeKeys(ctx context.Context, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	return infraAttributeKeys(ctx, "jobs", in)
}

// jobAttributeValues lists the values one job attribute key has taken, for
// building job filters.
func jobAttributeValues(ctx context.Context, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	return infraAttributeValues(ctx, "jobs", in)
}

// jobList lists Kubernetes jobs over a time range, each with the CPU and
// memory its pods used against request and limit, desired-successful, active,
// failed and successful pod counts, restarts and attributes; filterable,
// groupable and paginated.
func jobList(ctx context.Context, in *model.JobListRequest) (*O11yJobListOut, error) {
	out := new(O11yJobListOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/jobs/list", nil, in, out)
}

// ── infra_monitoring rollups ──────────────────────────────────────────────────

// infraHosts lists hosts with key infrastructure metrics — CPU, memory, I/O
// wait and disk usage percentages and 15-minute load — plus an
// active/inactive status from whether the host reported in the last ten
// minutes. Rows answer as 'list' under the default host.name grouping or
// 'grouped_list' under a custom groupBy; a metric with no data in the window
// answers -1. Filterable by expression and by status, orderable by any of the
// five metrics, paginated by offset and limit.
func infraHosts(ctx context.Context, in *inframonitoringtypes.PostableHosts) (*O11yInfraHostsOut, error) {
	out := new(O11yInfraHostsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/hosts", nil, in, out)
}

// infraPods lists Kubernetes pods with CPU and memory usage against request
// and limit, the pod's phase and its age, plus its namespace, node, owning
// workload and cluster attributes. Rows answer as 'list' under the default
// k8s.pod.uid grouping (each row one pod) or 'grouped_list' under a custom
// groupBy (each row aggregating its pods with per-phase counts); a metric with
// no data in the window answers -1. Filterable by expression, orderable by the
// six pod metrics, paginated by offset and limit.
func infraPods(ctx context.Context, in *inframonitoringtypes.PostablePods) (*O11yInfraPodsOut, error) {
	out := new(O11yInfraPodsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/pods", nil, in, out)
}

// infraNodes lists Kubernetes nodes with CPU and memory usage against
// allocatable capacity, per-group readiness counts and per-group phase counts
// for the pods scheduled on them. Rows answer as 'list' under the default
// k8s.node.name grouping (each row one node with its readiness condition) or
// 'grouped_list' under a custom groupBy; a metric with no data in the window
// answers -1. Filterable by expression, orderable by usage or allocatable,
// paginated by offset and limit.
func infraNodes(ctx context.Context, in *inframonitoringtypes.PostableNodes) (*O11yInfraNodesOut, error) {
	out := new(O11yInfraNodesOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/nodes", nil, in, out)
}

// infraNamespaces lists Kubernetes namespaces with the CPU and memory their
// pods used and per-group pod phase counts. Rows answer as 'list' under the
// default k8s.namespace.name grouping or 'grouped_list' under a custom
// groupBy, aggregating pods either way; a metric with no data in the window
// answers -1. Filterable by expression, orderable by cpu or memory, paginated
// by offset and limit.
func infraNamespaces(ctx context.Context, in *inframonitoringtypes.PostableNamespaces) (*O11yInfraNamespacesOut, error) {
	out := new(O11yInfraNamespacesOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/namespaces", nil, in, out)
}

// infraClusters lists Kubernetes clusters with CPU and memory usage against
// allocatable capacity summed over their nodes, plus per-group node readiness
// and pod phase counts. Rows answer as 'list' under the default
// k8s.cluster.name grouping or 'grouped_list' under a custom groupBy; a metric
// with no data in the window answers -1. Filterable by expression, orderable
// by usage or allocatable, paginated by offset and limit.
func infraClusters(ctx context.Context, in *inframonitoringtypes.PostableClusters) (*O11yInfraClustersOut, error) {
	out := new(O11yInfraClustersOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/clusters", nil, in, out)
}

// infraVolumes lists Kubernetes persistent volume claims with available,
// capacity and used bytes and inode counts, plus the claim's pod, namespace,
// node, statefulset and cluster attributes. Rows answer as 'list' under the
// default k8s.persistentvolumeclaim.name grouping or 'grouped_list' under a
// custom groupBy; a metric with no data in the window answers -1. Filterable
// by expression, orderable by the six volume metrics, paginated by offset and
// limit.
func infraVolumes(ctx context.Context, in *inframonitoringtypes.PostableVolumes) (*O11yInfraVolumesOut, error) {
	out := new(O11yInfraVolumesOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/pvcs", nil, in, out)
}

// infraDeployments lists Kubernetes deployments with the CPU and memory their
// pods used against request and limit, the latest desired and available
// replica counts, and per-group pod phase counts. Rows answer as 'list' under
// the default k8s.deployment.name grouping or 'grouped_list' under a custom
// groupBy; a metric with no data in the window answers -1. Filterable by
// expression, orderable by the pod metrics or the replica counts, paginated by
// offset and limit.
func infraDeployments(ctx context.Context, in *inframonitoringtypes.PostableDeployments) (*O11yInfraDeploymentsOut, error) {
	out := new(O11yInfraDeploymentsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/deployments", nil, in, out)
}

// infraStatefulSets lists Kubernetes statefulsets with the CPU and memory
// their pods used against request and limit, the latest desired and current
// replica counts, and per-group pod phase counts. Rows answer as 'list' under
// the default k8s.statefulset.name grouping or 'grouped_list' under a custom
// groupBy; a metric with no data in the window answers -1. Filterable by
// expression, orderable by the pod metrics or the replica counts, paginated by
// offset and limit.
func infraStatefulSets(ctx context.Context, in *inframonitoringtypes.PostableStatefulSets) (*O11yInfraStatefulSetsOut, error) {
	out := new(O11yInfraStatefulSetsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/statefulsets", nil, in, out)
}

// infraJobs lists Kubernetes jobs with the CPU and memory their pods used
// against request and limit, the latest desired-successful, active, failed and
// successful pod counters, and per-group pod phase counts — the phase counts
// are current state while the counters are cumulative over the job's life.
// Rows answer as 'list' under the default k8s.job.name grouping or
// 'grouped_list' under a custom groupBy; a metric with no data in the window
// answers -1. Filterable by expression, orderable by the pod metrics or the
// job counters, paginated by offset and limit.
func infraJobs(ctx context.Context, in *inframonitoringtypes.PostableJobs) (*O11yInfraJobsOut, error) {
	out := new(O11yInfraJobsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/jobs", nil, in, out)
}

// infraDaemonSets lists Kubernetes daemonsets with the CPU and memory their
// pods used against request and limit, the latest desired and current
// scheduled NODE counts (node counts, not pod counts), and per-group pod phase
// counts. Rows answer as 'list' under the default k8s.daemonset.name grouping
// or 'grouped_list' under a custom groupBy; a metric with no data in the
// window answers -1. Filterable by expression, orderable by the pod metrics or
// the node counts, paginated by offset and limit.
func infraDaemonSets(ctx context.Context, in *inframonitoringtypes.PostableDaemonSets) (*O11yInfraDaemonSetsOut, error) {
	out := new(O11yInfraDaemonSetsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/infra_monitoring/daemonsets", nil, in, out)
}

// infraChecks reports whether the metrics and attributes an infra-monitoring
// section needs are being received — for each collector receiver or processor
// involved, what is present and what is missing, with a user-facing message
// and a docs link per missing piece. Ready is true only when nothing is
// missing.
func infraChecks(ctx context.Context, in *O11yInfraChecksIn) (*O11yInfraChecksOut, error) {
	out := new(O11yInfraChecksOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/infra_monitoring/checks", query(
		"type", in.Type,
	), nil, out)
}

// ── the two shared reads ──────────────────────────────────────────────────────

// infraAttributeKeys is the ONE implementation behind the eleven
// */attribute_keys ops — the same runtime parser reads the same six query
// parameters for every resource face, so one function renders them.
func infraAttributeKeys(ctx context.Context, resource string, in *O11yInfraAttributeKeysIn) (*O11yInfraAttributeKeysOut, error) {
	out := new(O11yInfraAttributeKeysOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/"+resource+"/attribute_keys", query(
		"dataSource", in.DataSource,
		"aggregateOperator", in.AggregateOperator,
		"aggregateAttribute", in.AggregateAttribute,
		"searchText", in.SearchText,
		"tagType", in.TagType,
		"limit", in.Limit,
	), nil, out)
}

// infraAttributeValues is the ONE implementation behind the eleven
// */attribute_values ops, for the same reason.
func infraAttributeValues(ctx context.Context, resource string, in *O11yInfraAttributeValuesIn) (*O11yInfraAttributeValuesOut, error) {
	out := new(O11yInfraAttributeValuesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/"+resource+"/attribute_values", query(
		"dataSource", in.DataSource,
		"aggregateOperator", in.AggregateOperator,
		"aggregateAttribute", in.AggregateAttribute,
		"attributeKey", in.AttributeKey,
		"filterAttributeKeyDataType", in.FilterAttributeKeyDataType,
		"searchText", in.SearchText,
		"tagType", in.TagType,
		"limit", in.Limit,
	), nil, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────
//
// The GET inputs are named here because the runtime spells them as query
// parameters rather than as a type. No field carries validate: the runtime is
// the one place these are judged, so a bad or missing value answers with the
// status it has always answered with. The POST inputs are the runtime's own
// request types, imported, not mirrored.

// O11yInfraAttributeKeysIn selects which attribute keys to list. The type
// carries the O11y qualifier because these names enter a fleet-wide document,
// where one name may have exactly one shape.
type O11yInfraAttributeKeysIn struct {
	// DataSource is the telemetry the keys come from — metrics for the infra
	// faces. The runtime requires it.
	DataSource string `json:"dataSource"`
	// AggregateOperator is the aggregation the keys will be used under, e.g.
	// noop, count, avg. The runtime requires it for non-metrics sources.
	AggregateOperator string `json:"aggregateOperator"`
	// AggregateAttribute is the metric the keys must appear on.
	AggregateAttribute string `json:"aggregateAttribute"`
	// SearchText narrows the keys to those containing it.
	SearchText string `json:"searchText"`
	// TagType narrows the keys to one kind — tag or resource. Empty means all;
	// an invalid value reads as empty.
	TagType string `json:"tagType"`
	// Limit caps how many keys come back. Absent means 50.
	Limit int `json:"limit"`
}

// O11yInfraAttributeValuesIn selects which values of one attribute key to
// list.
type O11yInfraAttributeValuesIn struct {
	// DataSource is the telemetry the values come from — metrics for the infra
	// faces. The runtime requires it.
	DataSource string `json:"dataSource"`
	// AggregateOperator is the aggregation the values will be used under, e.g.
	// noop, count, avg. The runtime requires it for non-metrics sources.
	AggregateOperator string `json:"aggregateOperator"`
	// AggregateAttribute is the metric the values must appear on.
	AggregateAttribute string `json:"aggregateAttribute"`
	// AttributeKey is the key whose values to list.
	AttributeKey string `json:"attributeKey"`
	// FilterAttributeKeyDataType is the key's data type — string, int64,
	// float64 or bool. Empty means unspecified.
	FilterAttributeKeyDataType string `json:"filterAttributeKeyDataType"`
	// SearchText narrows the values to those containing it.
	SearchText string `json:"searchText"`
	// TagType narrows the search to one kind of key — tag or resource.
	TagType string `json:"tagType"`
	// Limit caps how many values come back. Absent means 50.
	Limit int `json:"limit"`
}

// O11yInfraChecksIn selects which infra-monitoring section the setup check
// runs for.
type O11yInfraChecksIn struct {
	// Type is the section to check — hosts, processes, pods, nodes,
	// deployments, daemonsets, statefulsets, jobs, namespaces, clusters or
	// volumes. Required.
	Type string `json:"type" validate:"required"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Each Out names the runtime's answer: the {status, data} envelope every read
// on this face has always answered with, around the runtime's own response
// type. Status is declared first because the runtime writes it first, and a
// typed op that changes the bytes is a break, not a migration.

// O11yInfraAttributeKeysOut is a page of attribute keys.
type O11yInfraAttributeKeysOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the keys.
	Data v3.FilterAttributeKeyResponse `json:"data,omitempty"`
}

// O11yInfraAttributeValuesOut is a page of one attribute key's values.
type O11yInfraAttributeValuesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the values, split by data type.
	Data v3.FilterAttributeValueResponse `json:"data,omitempty"`
}

// O11yHostListOut is a page of hosts.
type O11yHostListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the host records and the fleet's reporting state.
	Data model.HostListResponse `json:"data,omitempty"`
}

// O11yProcessListOut is a page of processes.
type O11yProcessListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the process records.
	Data model.ProcessListResponse `json:"data,omitempty"`
}

// O11yPodListOut is a page of pods.
type O11yPodListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the pod records.
	Data model.PodListResponse `json:"data,omitempty"`
}

// O11yPvcListOut is a page of persistent volume claims.
type O11yPvcListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the volume records.
	Data model.VolumeListResponse `json:"data,omitempty"`
}

// O11yNodeListOut is a page of nodes.
type O11yNodeListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the node records.
	Data model.NodeListResponse `json:"data,omitempty"`
}

// O11yNamespaceListOut is a page of namespaces.
type O11yNamespaceListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the namespace records.
	Data model.NamespaceListResponse `json:"data,omitempty"`
}

// O11yClusterListOut is a page of clusters.
type O11yClusterListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the cluster records.
	Data model.ClusterListResponse `json:"data,omitempty"`
}

// O11yDeploymentListOut is a page of deployments.
type O11yDeploymentListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the deployment records.
	Data model.DeploymentListResponse `json:"data,omitempty"`
}

// O11yDaemonSetListOut is a page of daemonsets.
type O11yDaemonSetListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the daemonset records.
	Data model.DaemonSetListResponse `json:"data,omitempty"`
}

// O11yStatefulSetListOut is a page of statefulsets.
type O11yStatefulSetListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the statefulset records.
	Data model.StatefulSetListResponse `json:"data,omitempty"`
}

// O11yJobListOut is a page of jobs.
type O11yJobListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the job records.
	Data model.JobListResponse `json:"data,omitempty"`
}

// O11yInfraHostsOut is a page of infra-monitoring host rows.
type O11yInfraHostsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Hosts `json:"data,omitempty"`
}

// O11yInfraPodsOut is a page of infra-monitoring pod rows.
type O11yInfraPodsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Pods `json:"data,omitempty"`
}

// O11yInfraNodesOut is a page of infra-monitoring node rows.
type O11yInfraNodesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Nodes `json:"data,omitempty"`
}

// O11yInfraNamespacesOut is a page of infra-monitoring namespace rows.
type O11yInfraNamespacesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Namespaces `json:"data,omitempty"`
}

// O11yInfraClustersOut is a page of infra-monitoring cluster rows.
type O11yInfraClustersOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Clusters `json:"data,omitempty"`
}

// O11yInfraVolumesOut is a page of infra-monitoring volume rows.
type O11yInfraVolumesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Volumes `json:"data,omitempty"`
}

// O11yInfraDeploymentsOut is a page of infra-monitoring deployment rows.
type O11yInfraDeploymentsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Deployments `json:"data,omitempty"`
}

// O11yInfraStatefulSetsOut is a page of infra-monitoring statefulset rows.
type O11yInfraStatefulSetsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.StatefulSets `json:"data,omitempty"`
}

// O11yInfraJobsOut is a page of infra-monitoring job rows.
type O11yInfraJobsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.Jobs `json:"data,omitempty"`
}

// O11yInfraDaemonSetsOut is a page of infra-monitoring daemonset rows.
type O11yInfraDaemonSetsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rows.
	Data inframonitoringtypes.DaemonSets `json:"data,omitempty"`
}

// O11yInfraChecksOut is one section's setup-check result.
type O11yInfraChecksOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds what is present, what is missing and whether the section is
	// ready.
	Data inframonitoringtypes.Checks `json:"data,omitempty"`
}

// ── the seam ──────────────────────────────────────────────────────────────────
