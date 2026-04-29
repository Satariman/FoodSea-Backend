package domain

import "fmt"

// Money represents an amount in kopecks (int64). No float arithmetic.
type Money struct {
	kopecks int64
}

func NewMoney(kopecks int64) Money { return Money{kopecks: kopecks} }

func (m Money) Kopecks() int64        { return m.kopecks }
func (m Money) Add(o Money) Money     { return Money{kopecks: m.kopecks + o.kopecks} }
func (m Money) Sub(o Money) Money     { return Money{kopecks: m.kopecks - o.kopecks} }
func (m Money) Mul(factor int) Money  { return Money{kopecks: m.kopecks * int64(factor)} }
func (m Money) IsZero() bool          { return m.kopecks == 0 }
func (m Money) String() string        { return fmt.Sprintf("%.2f ₽", float64(m.kopecks)/100) }
