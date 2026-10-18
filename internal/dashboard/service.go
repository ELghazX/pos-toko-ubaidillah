package dashboard

import (
	"context"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/ELghazX/pos-toko-ubaidillah/internal/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardData struct {
	TodaySales          float64
	TodayTransactions   int64
	TodayCOGS           float64
	TodayProfit         float64
	TotalUnpaidDebt     float64
	DebtCustomersCount  int64
	OverdueDebt         float64
	WeeklyTrend         []repo.GetWeeklySalesTrendRow
	TopProducts         []repo.GetTopProductsTodayOrWeekRow
	RecentSales         []repo.GetRecentDashboardSalesRow
	LowStockProducts    []repo.GetDashboardLowStockProductsRow
	CompanyName         string
}

type Service interface {
	GetDashboardData(ctx context.Context) (*DashboardData, error)
}

type service struct {
	repo repo.Querier
	db   *pgxpool.Pool
}

func NewService(r repo.Querier, db *pgxpool.Pool) Service {
	return &service{repo: r, db: db}
}

func (s *service) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	salesMetrics, err := s.repo.GetTodaySalesMetrics(ctx)
	if err != nil {
		return nil, err
	}

	receivables, err := s.repo.GetTodayReceivablesMetrics(ctx)
	if err != nil {
		return nil, err
	}

	weeklyTrend, err := s.repo.GetWeeklySalesTrend(ctx)
	if err != nil {
		weeklyTrend = []repo.GetWeeklySalesTrendRow{}
	}

	topProducts, err := s.repo.GetTopProductsTodayOrWeek(ctx)
	if err != nil {
		topProducts = []repo.GetTopProductsTodayOrWeekRow{}
	}

	recentSales, err := s.repo.GetRecentDashboardSales(ctx)
	if err != nil {
		recentSales = []repo.GetRecentDashboardSalesRow{}
	}

	lowStock, err := s.repo.GetDashboardLowStockProducts(ctx)
	if err != nil {
		lowStock = []repo.GetDashboardLowStockProductsRow{}
	}

	company, _ := s.repo.GetCompanyConfig(ctx)
	compName := company.Name
	if compName == "" {
		compName = "Toko Ubaidillah"
	}

	todaySales := utils.NumericToFloat(salesMetrics.TotalSales)
	todayCOGS := utils.NumericToFloat(salesMetrics.TotalCogs)
	todayProfit := todaySales - todayCOGS

	return &DashboardData{
		TodaySales:         todaySales,
		TodayTransactions:  salesMetrics.TotalTransactions,
		TodayCOGS:          todayCOGS,
		TodayProfit:        todayProfit,
		TotalUnpaidDebt:    utils.NumericToFloat(receivables.TotalUnpaidDebt),
		DebtCustomersCount: receivables.CustomersCount,
		OverdueDebt:        utils.NumericToFloat(receivables.OverdueDebt),
		WeeklyTrend:        weeklyTrend,
		TopProducts:        topProducts,
		RecentSales:        recentSales,
		LowStockProducts:   lowStock,
		CompanyName:        compName,
	}, nil
}
