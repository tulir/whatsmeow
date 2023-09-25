package sqlstore

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type ClientInstance struct {
	clients map[string]*whatsmeow.Client
	dbPool  *pgxpool.Pool
	log     waLog.Logger
}
