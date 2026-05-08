package apm

import (
	"context"

	"github.com/getsentry/sentry-go"
)

func GetTransactionID(ctx context.Context) string {
	tx := sentry.TransactionFromContext(ctx)
	if tx == nil {
		return ""
	}

	return tx.ToSentryTrace()
}
