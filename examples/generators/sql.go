package generators

import (
	"database/sql"
	"fmt"
	// mysql
	// _ "github.com/go-sql-driver/mysql"
	// postgres
	// _ "github.com/lib/pq"
)

type DBGenerator struct {
	db       *sql.DB
	query    string
	pageSize int
	offset   int
	hasMore  bool
}

func NewDBGenerator(db *sql.DB, query string, pageSize int) func() ([]map[string]any, bool, error) {
	gen := &DBGenerator{
		db:       db,
		query:    query,
		pageSize: pageSize,
		offset:   0,
		hasMore:  true,
	}
	return gen.fetch
}

func (g *DBGenerator) fetch() ([]map[string]any, bool, error) {
	if !g.hasMore {
		return nil, false, nil
	}

	rows, err := g.db.Query(fmt.Sprintf("%s LIMIT %d OFFSET %d", g.query, g.pageSize, g.offset))
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, false, err
	}

	var results []map[string]any
	for rows.Next() {
		scanArgs := make([]interface{}, len(columns))
		for i := range scanArgs {
			var v interface{}
			scanArgs[i] = &v
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, false, err
		}

		rowMap := make(map[string]any)
		for i, colName := range columns {
			rowMap[colName] = *(scanArgs[i].(*interface{}))
		}
		results = append(results, rowMap)
	}

	g.hasMore = len(results) == g.pageSize
	g.offset += g.pageSize

	return results, g.hasMore, nil
}

// func main() {
// 	// mysql
// 	db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/dbname")
// 	// postgres
// 	db, err := sql.Open("postgres", "user=password dbname=yourdb sslmode=disable")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer db.Close()

// 	dbGen := NewDBGenerator(db, "SELECT * FROM your_table", 10)
// 	for {
// 		data, hasMore, err := dbGen()
// 		if err != nil {
// 			fmt.Println("Error:", err)
// 			break
// 		}
// 		if !hasMore {
// 			break
// 		}
// 		fmt.Println("Fetched:", data)
// 	}
// }
