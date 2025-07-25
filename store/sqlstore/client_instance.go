package sqlstore

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type ClientBundle struct {
	Qr     string
	Client *whatsmeow.Client
}

type ClientInstance struct {
	Clients map[string]*ClientBundle
	DbPool  *pgxpool.Pool
	Log     waLog.Logger
}
