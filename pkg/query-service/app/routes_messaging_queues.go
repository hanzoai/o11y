package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountMessagingQueues registers the messaging-queues surface: the queue
// overview plus the Kafka onboarding, partition-latency, consumer-lag,
// topic-throughput and span-evaluation groups. 14 routes on nested subrouters
// under /v1/o11y/messaging-queues.
func (aH *APIHandler) mountMessagingQueues(router *mux.Router, am *middleware.AuthZ) {
	// Main messaging queues router
	messagingQueuesRouter := router.PathPrefix("/v1/o11y/messaging-queues").Subrouter()

	// Queue Overview route
	messagingQueuesRouter.HandleFunc("/queue-overview", am.ViewAccess(aH.getQueueOverview)).Methods(http.MethodPost)

	// -------------------------------------------------
	// Kafka-specific routes
	kafkaRouter := messagingQueuesRouter.PathPrefix("/kafka").Subrouter()

	onboardingRouter := kafkaRouter.PathPrefix("/onboarding").Subrouter()

	onboardingRouter.HandleFunc("/producers", am.ViewAccess(aH.onboardProducers)).Methods(http.MethodPost)
	onboardingRouter.HandleFunc("/consumers", am.ViewAccess(aH.onboardConsumers)).Methods(http.MethodPost)
	onboardingRouter.HandleFunc("/kafka", am.ViewAccess(aH.onboardKafka)).Methods(http.MethodPost)

	partitionLatency := kafkaRouter.PathPrefix("/partition-latency").Subrouter()

	partitionLatency.HandleFunc("/overview", am.ViewAccess(aH.getPartitionOverviewLatencyData)).Methods(http.MethodPost)
	partitionLatency.HandleFunc("/consumer", am.ViewAccess(aH.getConsumerPartitionLatencyData)).Methods(http.MethodPost)

	consumerLagRouter := kafkaRouter.PathPrefix("/consumer-lag").Subrouter()

	consumerLagRouter.HandleFunc("/producer-details", am.ViewAccess(aH.getProducerData)).Methods(http.MethodPost)
	consumerLagRouter.HandleFunc("/consumer-details", am.ViewAccess(aH.getConsumerData)).Methods(http.MethodPost)
	consumerLagRouter.HandleFunc("/network-latency", am.ViewAccess(aH.getNetworkData)).Methods(http.MethodPost)

	topicThroughput := kafkaRouter.PathPrefix("/topic-throughput").Subrouter()

	topicThroughput.HandleFunc("/producer", am.ViewAccess(aH.getProducerThroughputOverview)).Methods(http.MethodPost)
	topicThroughput.HandleFunc("/producer-details", am.ViewAccess(aH.getProducerThroughputDetails)).Methods(http.MethodPost)
	topicThroughput.HandleFunc("/consumer", am.ViewAccess(aH.getConsumerThroughputOverview)).Methods(http.MethodPost)
	topicThroughput.HandleFunc("/consumer-details", am.ViewAccess(aH.getConsumerThroughputDetails)).Methods(http.MethodPost)

	spanEvaluation := kafkaRouter.PathPrefix("/span").Subrouter()

	spanEvaluation.HandleFunc("/evaluation", am.ViewAccess(aH.getProducerConsumerEval)).Methods(http.MethodPost)
}
