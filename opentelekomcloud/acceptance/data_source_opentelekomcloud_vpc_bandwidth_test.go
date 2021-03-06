package acceptance

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestAccBandWidthDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-test-%s", acctest.RandString(5))
	dataName := "data.opentelekomcloud_vpc_bandwidth.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccBandWidthDataSource_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckBandWidthDataSourceExists(dataName),
					resource.TestCheckResourceAttr(dataName, "name", rName),
					resource.TestCheckResourceAttr(dataName, "size", "10"),
				),
			},
		},
	})
}

func testAccCheckBandWidthDataSourceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("root module has no resource called %s", n)
		}

		bandwidthRs, ok := s.RootModule().Resources["opentelekomcloud_vpc_bandwidth.test"]
		if !ok {
			return fmt.Errorf("can't find opentelekomcloud_vpc_bandwidth.test in state")
		}

		attr := rs.Primary.Attributes
		if attr["id"] != bandwidthRs.Primary.Attributes["id"] {
			return fmt.Errorf("attribute 'id' expected %s; got %s",
				bandwidthRs.Primary.Attributes["id"], attr["id"])
		}

		return nil
	}
}

func testAccBandWidthDataSource_basic(rName string) string {
	return fmt.Sprintf(`
resource "opentelekomcloud_vpc_bandwidth" "test" {
	name = "%s"
	size = 10
}

data "opentelekomcloud_vpc_bandwidth" "test" {
  name = opentelekomcloud_vpc_bandwidth.test.name
}
`, rName)
}
