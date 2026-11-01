package domain

import (
	"github.com/dujiao-next/internal/shared/jsonmap"
	"testing"
	"time"
)

func TestOrderBasisSurvivesDatabaseBookkeepingButBindsObligations(t *testing.T) {
	o := splitOrder()
	p := Payment{ID: 1, OrderID: o.ID}
	before := OrderBasis(p, o)
	o.Children[0].Items[0].CreatedAt = time.Now().Truncate(time.Microsecond)
	o.Children[0].Items[0].UpdatedAt = time.Now()
	o.Children[0].Items[0].PromotionName = "transient display label"
	o.Children[0].Items[0].InstructionsJSON = jsonmap.JSON{}
	if OrderBasis(p, o) != before {
		t.Fatal("database representation changed payment authorization")
	}
	o.Children[0].Items[0].Quantity++
	if OrderBasis(p, o) == before {
		t.Fatal("changed delivery quantity preserved authorization")
	}
	o.Children[0].Items[0].Quantity--
	o.Children[0].Items[0].InstructionsJSON = jsonmap.JSON{"en": "different fulfillment instructions"}
	if OrderBasis(p, o) == before {
		t.Fatal("changed delivery snapshot preserved authorization")
	}
}
