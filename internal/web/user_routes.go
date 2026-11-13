package web

import "regexp"

// Keep this allowlist aligned with frontend/user/src/router/index.ts and the
// frontend gateway. Dynamic public documents are normally handled before it.
var userRoute = regexp.MustCompile(`^/(?:products(?:/[^/]+)?|categories/[^/]+|blog(?:/[^/]+)?|notice|about|terms|privacy|redeem|cart|checkout|pay|me(?:/(?:profile|security|orders|wallet|gift-cards|api|affiliate|reseller))?|reseller(?:/(?:apply|domains|site|products|orders(?:/[^/]+)?|finance|ledger|withdraws))?|orders/[^/]+|recharge-orders/[^/]+|guest/orders(?:/[^/]+)?|auth/(?:login|register|forgot|(?:google|telegram)/callback))/?$`)

func isUserRoute(path string) bool {
	return path == "/" || userRoute.MatchString(path)
}

var privateUserRoute = regexp.MustCompile(`^/(?:redeem|cart|checkout|pay|me|reseller|orders|recharge-orders|guest|auth)(?:/|$)`)
