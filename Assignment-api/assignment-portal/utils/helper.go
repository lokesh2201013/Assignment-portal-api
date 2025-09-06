package utils

import (
	"strings"
)

func CleanSQL(sql string) string {
    sql = strings.TrimSpace(sql)
    sql = strings.TrimPrefix(sql, "```sql")
    sql = strings.TrimPrefix(sql, "```")
    sql = strings.TrimSuffix(sql, "```")
    return strings.TrimSpace(sql)
}