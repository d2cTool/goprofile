package observability

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterPoolCollector(pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	collectors := []prometheus.Collector{
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_acquired",
			Help: "PostgreSQL pool connections currently acquired",
		}, func() float64 { return float64(pool.Stat().AcquiredConns()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_idle",
			Help: "PostgreSQL pool idle connections",
		}, func() float64 { return float64(pool.Stat().IdleConns()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_pool_max",
			Help: "PostgreSQL pool max connections",
		}, func() float64 { return float64(pool.Stat().MaxConns()) }),
	}
	for _, c := range collectors {
		if err := prometheus.Register(c); err != nil {
			var alreadyRegistered prometheus.AlreadyRegisteredError
			if !errors.As(err, &alreadyRegistered) {
				return err
			}
		}
	}
	return nil
}
