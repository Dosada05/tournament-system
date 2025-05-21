package repositories

import (
	"database/sql"
	"fmt"
)

func checkAffectedRows(result sql.Result, notFoundError error) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return notFoundError // Возвращаем переданную ошибку "не найдено"
	}
	return nil
}
