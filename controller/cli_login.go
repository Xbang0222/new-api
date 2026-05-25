package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"

	"github.com/gin-gonic/gin"
)

// CliLoginExchange swaps the caller's session for a fresh access token,
// for use by the newapi CLI's browser-login flow. The browser-side page
// arrives here with a valid session cookie (the user just signed in or
// was already signed in) and a state nonce that the CLI generated. We
// echo the state back so the loopback endpoint can verify the response
// came from the originating CLI invocation; we do NOT validate state
// server-side — that is the CLI's job.
func CliLoginExchange(c *gin.Context) {
	id := c.GetInt("id")
	if id == 0 {
		// session missing — defensive: session-auth middleware on the
		// /api/user group should have already rejected this.
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "not logged in",
		})
		return
	}

	var req struct {
		State string `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	token, err := issueAccessTokenForUser(id)
	if err != nil {
		switch {
		case errors.Is(err, errIssueAccessTokenGenerateFailed):
			common.ApiErrorI18n(c, i18n.MsgGenerateFailed)
		case errors.Is(err, errIssueAccessTokenDuplicate):
			common.ApiErrorI18n(c, i18n.MsgUuidDuplicate)
		default:
			common.ApiError(c, err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"access_token": token,
			"user_id":      id,
			"state":        req.State,
		},
	})
}
