package db

import (
	"context"
	"database/sql"
)

// execDesfazendoEmErro executa query numa conexão dedicada do pool e, se
// falhar, emite ROLLBACK nessa mesma conexão antes de devolvê-la.
//
// Motivo: um Exec com várias instruções ("BEGIN; ...; COMMIT;") em que uma
// do meio falha deixa a transação aberta na conexão; database/sql a
// devolve ao pool assim, e as escritas seguintes que pegam essa conexão
// caem dentro da transação pendurada (nunca confirmada: somem no fim do
// processo) ou esperam a trava e falham. O ROLLBACK sem transação ativa
// dá erro no SGBD, que é ignorado de propósito.
func execDesfazendoEmErro(db *sql.DB, query string, args ...any) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, query, args...); err != nil {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
		return err
	}
	return nil
}
