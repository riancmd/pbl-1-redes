package server

import (
	"encoding/json"
	"errors"
	"os"
)

func NewAccountStorage(filename string) *AccountStorage {
	return &AccountStorage{
		filename: filename,
	}
}

func (accStorage *AccountStorage) LoadAccount(Username string) (*User, error) {
	accStorage.mutex.RLock()
	defer accStorage.mutex.RUnlock()

	accounts, error := accStorage.loadAllAccounts()
	if error != nil {
		return nil, error
	}

	for _, account := range accounts {
		if account.Username == Username {
			return &account, nil
		}
	}

	return nil, errors.New("conta não encontrada")
}

func (accStorage *AccountStorage) SaveAccount(account *User) error {
	accStorage.mutex.Lock()
	defer accStorage.mutex.Unlock()

	accounts, error := accStorage.loadAllAccounts()
	if error != nil {
		accounts = []User{}
	}

	// Verificar se já existe
	for i, existing := range accounts {
		if existing.Username == account.Username {
			accounts[i] = *account
			return accStorage.saveAllAccounts(accounts)
		}
	}

	// Adicionar nova conta
	accounts = append(accounts, *account)
	return accStorage.saveAllAccounts(accounts)
}

func (accStorage *AccountStorage) AccountExists(Username string) bool {
	account, _ := accStorage.LoadAccount(Username)
	return account != nil
}

func (accStorage *AccountStorage) loadAllAccounts() ([]User, error) {
	if _, error := os.Stat(accStorage.filename); os.IsNotExist(error) {
		return []User{}, nil
	}

	data, error := os.ReadFile(accStorage.filename)
	if error != nil {
		return nil, error
	}

	var accounts []User
	error = json.Unmarshal(data, &accounts)
	if error != nil {
		return nil, error
	}

	return accounts, nil
}

func (accStorage *AccountStorage) saveAllAccounts(accounts []User) error {
	data, error := json.MarshalIndent(accounts, "", "  ")
	if error != nil {
		return error
	}

	return os.WriteFile(accStorage.filename, data, 0644)
}
