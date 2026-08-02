package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountMessagingQueues registers the messaging-queues surface: the queue
// overview plus the Kafka onboarding, partition-latency, consumer-lag,
// topic-throughput and span-evaluation groups. 14 routes on nested subrouters
// under /v1/o11y/messaging-queues.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. These registrations stay: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handlers through the typed ops in the repo root's apm.go (queueOverview,
// the onboarding, partition-latency, consumer-lag, topic-throughput and span
// ops), which relay here — deleting either half drops one of the two
// deployments. The ViewAccess gate on each route is unchanged: it runs here,
// one layer in.
func (aH *APIHandler) mountMessagingQueues(router routing.Router, am *middleware.AuthZ) {
	// Main messaging queues router
	messagingQueuesRouter := router.Group("/v1/o11y/messaging-queues")

	// Queue Overview route
	messagingQueuesRouter.Post("/queue-overview", am.ViewAccess(aH.getQueueOverview))

	// -------------------------------------------------
	// Kafka-specific routes
	kafkaRouter := messagingQueuesRouter.Group("/kafka")

	onboardingRouter := kafkaRouter.Group("/onboarding")

	onboardingRouter.Post("/producers", am.ViewAccess(aH.onboardProducers))
	onboardingRouter.Post("/consumers", am.ViewAccess(aH.onboardConsumers))
	onboardingRouter.Post("/kafka", am.ViewAccess(aH.onboardKafka))

	partitionLatency := kafkaRouter.Group("/partition-latency")

	partitionLatency.Post("/overview", am.ViewAccess(aH.getPartitionOverviewLatencyData))
	partitionLatency.Post("/consumer", am.ViewAccess(aH.getConsumerPartitionLatencyData))

	consumerLagRouter := kafkaRouter.Group("/consumer-lag")

	consumerLagRouter.Post("/producer-details", am.ViewAccess(aH.getProducerData))
	consumerLagRouter.Post("/consumer-details", am.ViewAccess(aH.getConsumerData))
	consumerLagRouter.Post("/network-latency", am.ViewAccess(aH.getNetworkData))

	topicThroughput := kafkaRouter.Group("/topic-throughput")

	topicThroughput.Post("/producer", am.ViewAccess(aH.getProducerThroughputOverview))
	topicThroughput.Post("/producer-details", am.ViewAccess(aH.getProducerThroughputDetails))
	topicThroughput.Post("/consumer", am.ViewAccess(aH.getConsumerThroughputOverview))
	topicThroughput.Post("/consumer-details", am.ViewAccess(aH.getConsumerThroughputDetails))

	spanEvaluation := kafkaRouter.Group("/span")

	spanEvaluation.Post("/evaluation", am.ViewAccess(aH.getProducerConsumerEval))
}
