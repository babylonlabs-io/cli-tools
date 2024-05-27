package services

import (
	"github.com/babylonchain/cli-tools/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

type PipelineMetrics struct {
	SuccessSigningReqs         *prometheus.CounterVec
	FailedSigningReqs          *prometheus.CounterVec
	SuccessfulSentTransactions prometheus.Counter
	FailureSentTransactions    prometheus.Counter
	Config                     *config.MetricsConfig
}

func NewPipelineMetrics(cfg *config.MetricsConfig) *PipelineMetrics {
	return &PipelineMetrics{
		SuccessSigningReqs: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "number_of_successful_signing_requests",
				Help: "How many signing requests to given covenant were successful",
			},
			[]string{"covenant_pk"},
		),
		FailedSigningReqs: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "number_of_failed_signing_requests",
				Help: "How many signing requests to given covenant failed",
			},
			[]string{"covenant_pk"},
		),
		SuccessfulSentTransactions: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "number_of_successful_unbonding_transactions",
				Help: "How many transactions were successfully sent to the network",
			},
		),
		FailureSentTransactions: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "number_of_failed_unbonding_transactions",
				Help: "How many transactions failed to be sent to the network",
			},
		),
		Config: cfg,
	}
}

func (pm *PipelineMetrics) RecordSuccessSigningRequest(covenantPk string) {
	pm.SuccessSigningReqs.WithLabelValues(covenantPk).Inc()
}

func (pm *PipelineMetrics) RecordFailedSigningRequest(covenantPk string) {
	pm.FailedSigningReqs.WithLabelValues(covenantPk).Inc()
}

func (pm *PipelineMetrics) RecordSentUnbondingTransaction() {
	pm.SuccessfulSentTransactions.Inc()
}

func (pm *PipelineMetrics) RecordFailedUnbodingTransaction() {
	pm.FailureSentTransactions.Inc()
}
