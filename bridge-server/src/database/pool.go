package database

import (
	// internal packages
	context "context"
	// external packages
	pgxpool "github.com/jackc/pgx/v4/pgxpool"
	// local packages
	env "bitaminco/support-whatsapp-bridge/src/environment"
)

///////////////////
//   instances   //
///////////////////

var (
	Postgres *pgxpool.Pool
	Context  context.Context = context.Background()
	err      error
)

///////////////////////
//   db connection   //
///////////////////////

func Connect() *pgxpool.Pool {
	Postgres, err = pgxpool.Connect(Context, env.DATABASE_URL)
	if err != nil {
		env.ErrorLogger.Panic(err)
	}
	// postgres instance
	return Postgres
}
