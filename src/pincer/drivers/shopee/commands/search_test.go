package commands

import (
	"testing"

	"github.com/prathan/pincer/src/pincer/core"
)

func TestParseSearchResultsFiltersNonProducts(t *testing.T) {
	xml := `<?xml version='1.0' encoding='UTF-8' standalone='yes' ?>
<hierarchy rotation="0">
  <node index="0" text="" resource-id="" class="android.widget.FrameLayout" package="com.shopee.th" bounds="[0,0][1080,2400]">
    <node index="0" text="" resource-id="" class="android.view.ViewGroup" package="com.shopee.th" clickable="true" bounds="[0,0][1080,500]">
      <node index="0" text="USB-C Braided Cable 2M Fast Charge" resource-id="" class="android.widget.TextView" package="com.shopee.th" bounds="[50,50][900,120]"/>
      <node index="1" text="฿129" resource-id="" class="android.widget.TextView" package="com.shopee.th" bounds="[50,140][250,200]"/>
      <node index="2" text="-15%" resource-id="" class="android.widget.TextView" package="com.shopee.th" bounds="[260,140][360,200]"/>
      <node index="3" text="2.1k sold" resource-id="" class="android.widget.TextView" package="com.shopee.th" bounds="[370,140][520,200]"/>
    </node>
    <node index="1" text="จังหวัดกรุงเทพมหานคร" resource-id="" class="android.widget.TextView" package="com.shopee.th" bounds="[0,600][400,660]"/>
  </node>
</hierarchy>`

	finder, err := core.NewElementFinderFromXML([]byte(xml))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	products := parseSearchResults(finder)
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	if products[0].Name != "USB-C Braided Cable 2M Fast Charge" {
		t.Fatalf("unexpected product name %q", products[0].Name)
	}
	if products[0].Price != "฿129" {
		t.Fatalf("expected price to be captured, got %q", products[0].Price)
	}
	if products[0].Discount != "-15%" {
		t.Fatalf("expected discount to be captured, got %q", products[0].Discount)
	}
	if products[0].Sold != "2.1k sold" {
		t.Fatalf("expected sold count to be captured, got %q", products[0].Sold)
	}
}
