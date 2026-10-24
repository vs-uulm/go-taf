package leftturnassist

import (
	"github.com/vs-uulm/go-taf/pkg/trustmodel"
	leftturnassist_v0_1_0 "github.com/vs-uulm/go-taf/plugins/trustmodels/leftturnassist/v0_1_0"
)

func init() {
	trustmodel.RegisterTemplate(leftturnassist_v0_1_0.CreateTrustModelTemplate("LTA", "0.1.0"))
}
