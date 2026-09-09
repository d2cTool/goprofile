package observability

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterPoolCollector(pool *pgxpool.Pool) {
	if pool == nil {
		return
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
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic(err)
			}
		}
	}
}
