package repository

import (
	"errors"
	"time"

	"github.com/ccxt-simulator/internal/models"
	"gorm.io/gorm"
)

var (
	ErrOrderNotFound = errors.New("order not found")
)

// OrderRepository handles order data access
type OrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository creates a new OrderRepository
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create creates a new order
func (r *OrderRepository) Create(order *models.Order) error {
	return r.db.Create(order).Error
}

// GetByID retrieves an order by ID
func (r *OrderRepository) GetByID(id uint) (*models.Order, error) {
	var order models.Order
	result := r.db.First(&order, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, result.Error
	}
	return &order, nil
}

// GetByClientOrderID retrieves an order by client order ID
func (r *OrderRepository) GetByClientOrderID(accountID uint, clientOrderID string) (*models.Order, error) {
	var order models.Order
	result := r.db.Where("account_id = ? AND client_order_id = ?", accountID, clientOrderID).First(&order)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, result.Error
	}
	return &order, nil
}

// GetByAccountID retrieves all orders for an account
func (r *OrderRepository) GetByAccountID(accountID uint) ([]models.Order, error) {
	var orders []models.Order
	result := r.db.Where("account_id = ?", accountID).Order("created_at DESC").Find(&orders)
	return orders, result.Error
}

// GetByAccountIDPaginated retrieves orders with pagination
func (r *OrderRepository) GetByAccountIDPaginated(accountID uint, page, pageSize int) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	if err := r.db.Model(&models.Order{}).Where("account_id = ?", accountID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	result := r.db.Where("account_id = ?", accountID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&orders)

	return orders, total, result.Error
}

// GetOpenOrders retrieves all open orders for an account
func (r *OrderRepository) GetOpenOrders(accountID uint) ([]models.Order, error) {
	var orders []models.Order
	result := r.db.Where("account_id = ? AND status IN ?", accountID, []models.OrderStatus{
		models.OrderStatusNew,
		models.OrderStatusPartiallyFilled,
	}).Find(&orders)
	return orders, result.Error
}

// GetOpenOrdersBySymbol retrieves open orders for a specific symbol
func (r *OrderRepository) GetOpenOrdersBySymbol(accountID uint, symbol string) ([]models.Order, error) {
	var orders []models.Order
	result := r.db.Where("account_id = ? AND symbol = ? AND status IN ?", accountID, symbol, []models.OrderStatus{
		models.OrderStatusNew,
		models.OrderStatusPartiallyFilled,
	}).Find(&orders)
	return orders, result.Error
}

// GetPendingStopOrders retrieves pending stop loss and take profit orders
func (r *OrderRepository) GetPendingStopOrders(accountID uint) ([]models.Order, error) {
	var orders []models.Order
	result := r.db.Where("account_id = ? AND status = ? AND type IN ?", accountID, models.OrderStatusNew, []models.OrderType{
		models.OrderTypeStopLoss,
		models.OrderTypeTakeProfit,
		models.OrderTypeStopMarket,
	}).Find(&orders)
	return orders, result.Error
}

// Update updates an order
func (r *OrderRepository) Update(order *models.Order) error {
	return r.db.Save(order).Error
}

// UpdateStatus updates order status
func (r *OrderRepository) UpdateStatus(id uint, status models.OrderStatus) error {
	return r.db.Model(&models.Order{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

// CancelOrder cancels an order
func (r *OrderRepository) CancelOrder(id uint) error {
	return r.db.Model(&models.Order{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     models.OrderStatusCanceled,
		"updated_at": time.Now(),
	}).Error
}

// CancelAllOpenOrders cancels all open orders for an account
func (r *OrderRepository) CancelAllOpenOrders(accountID uint) (int64, error) {
	result := r.db.Model(&models.Order{}).
		Where("account_id = ? AND status IN ?", accountID, []models.OrderStatus{
			models.OrderStatusNew,
			models.OrderStatusPartiallyFilled,
		}).
		Updates(map[string]interface{}{
			"status":     models.OrderStatusCanceled,
			"updated_at": time.Now(),
		})
	return result.RowsAffected, result.Error
}

// CancelAllOpenOrdersBySymbol cancels all open orders for a symbol
func (r *OrderRepository) CancelAllOpenOrdersBySymbol(accountID uint, symbol string) (int64, error) {
	result := r.db.Model(&models.Order{}).
		Where("account_id = ? AND symbol = ? AND status IN ?", accountID, symbol, []models.OrderStatus{
			models.OrderStatusNew,
			models.OrderStatusPartiallyFilled,
		}).
		Updates(map[string]interface{}{
			"status":     models.OrderStatusCanceled,
			"updated_at": time.Now(),
		})
	return result.RowsAffected, result.Error
}

// GetOrdersAfter retrieves orders created after a specific time
func (r *OrderRepository) GetOrdersAfter(accountID uint, after time.Time, limit int) ([]models.Order, error) {
	var orders []models.Order
	result := r.db.Where("account_id = ? AND created_at > ?", accountID, after).
		Order("created_at DESC").
		Limit(limit).
		Find(&orders)
	return orders, result.Error
}

// GetOpenOrdersByTypes retrieves open orders of specific types
func (r *OrderRepository) GetOpenOrdersByTypes(accountID uint, orderTypes []models.OrderType) ([]models.Order, error) {
	var orders []models.Order
	result := r.db.Where("account_id = ? AND status IN ? AND type IN ?", accountID,
		[]models.OrderStatus{models.OrderStatusNew, models.OrderStatusPartiallyFilled},
		orderTypes).Find(&orders)
	return orders, result.Error
}

// GetOpenOrdersBySymbolAndTypes retrieves open orders of specific types for a symbol
func (r *OrderRepository) GetOpenOrdersBySymbolAndTypes(accountID uint, symbol string, orderTypes []models.OrderType) ([]models.Order, error) {
	var orders []models.Order
	result := r.db.Where("account_id = ? AND symbol = ? AND status IN ? AND type IN ?", accountID, symbol,
		[]models.OrderStatus{models.OrderStatusNew, models.OrderStatusPartiallyFilled},
		orderTypes).Find(&orders)
	return orders, result.Error
}

// CancelOpenOrdersByTypes cancels open orders of specific types
func (r *OrderRepository) CancelOpenOrdersByTypes(accountID uint, symbol string, orderTypes []models.OrderType) (int64, error) {
	query := r.db.Model(&models.Order{}).
		Where("account_id = ? AND status IN ? AND type IN ?", accountID,
			[]models.OrderStatus{models.OrderStatusNew, models.OrderStatusPartiallyFilled},
			orderTypes)

	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}

	result := query.Updates(map[string]interface{}{
		"status":     models.OrderStatusCanceled,
		"updated_at": time.Now(),
	})
	return result.RowsAffected, result.Error
}
