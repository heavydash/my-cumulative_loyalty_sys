package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"testing"
	"time"
)

func TestUserStorage_Create(t *testing.T) {
	ctx := context.Background()

	fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("success", func(t *testing.T) {
		// Создаём новый мок с ТОЧНЫМ матчем SQL (QueryMatcherEqual)
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewUserStorage(db)

		login := "testuser"
		password := "secret123"

		realHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		require.NoError(t, err)

		expectedSQL := "INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id, login, password_hash, created_at"

		// Ожидаем вызов QueryRowContext с точным SQL и аргументами
		mock.ExpectQuery(expectedSQL).
			WithArgs(login, sqlmock.AnyArg()). // пароль хэшируется, поэтому AnyArg для hash
			WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password_hash", "created_at"}).
				AddRow(int64(1), login, string(realHash), fixedTime)) // значения, которые вернёт Scan

		user, err := storage.Create(ctx, login, password)

		require.NoError(t, err)
		assert.Equal(t, int64(1), user.ID)
		assert.Equal(t, login, user.Login)
		// Проверяем, что пароль действительно хэшировался bcrypt
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)))

		// Проверяем, что все ожидания мока выполнены
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewUserStorage(db)

		login := "user"
		password := "pass"

		mock.ExpectQuery(`INSERT INTO users (login, password_hash) VALUES ($1, $2)
 RETURNING id, login, password_hash, created_at`).
			WithArgs(login, sqlmock.AnyArg()).
			WillReturnError(sql.ErrConnDone) // любая ошибка БД

		user, err := storage.Create(ctx, login, password)

		assert.Nil(t, user)
		assert.Error(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUserStorage_GetByLogin(t *testing.T) {
	ctx := context.Background()
	fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("success", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewUserStorage(db)

		login := "testuser"
		hash := "$2a$10$examplehash"

		mock.ExpectQuery("SELECT id, login, password_hash, created_at FROM users WHERE login = $1").
			WithArgs(login).
			WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password_hash", "created_at"}).
				AddRow(int64(42), login, hash, fixedTime))

		user, err := storage.GetByLogin(ctx, login)

		require.NoError(t, err)
		assert.Equal(t, int64(42), user.ID)
		assert.Equal(t, login, user.Login)
		assert.Equal(t, hash, user.PasswordHash)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user not found", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewUserStorage(db)

		login := "unknown"

		mock.ExpectQuery(`SELECT id, login, password_hash, created_at FROM users WHERE
                                                           login = $1`).
			WithArgs(login).
			WillReturnError(sql.ErrNoRows) // ошибка возвращаемая в коде

		user, err := storage.GetByLogin(ctx, login)

		assert.Nil(t, user)
		assert.ErrorIs(t, err, sql.ErrNoRows) // проверяем точную ошибку

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewUserStorage(db)

		login := "user"

		mock.ExpectQuery(`SELECT id, login, password_hash, created_at FROM users WHERE
                                                           login = $1`).
			WithArgs(login).
			WillReturnError(errors.New("some db error")) // другая ошибка БД

		user, err := storage.GetByLogin(ctx, login)

		assert.Nil(t, user)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, sql.ErrNoRows) // не должна быть ErrNoRows

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
