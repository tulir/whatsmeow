package sqlstore

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type ClientInstance struct {
	Clients map[string]*whatsmeow.Client
	DbPool  *pgxpool.Pool
	Qr      string
	Log     waLog.Logger
}
