package db

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Sqlit is a Sqlit helper for executing commands.
type Sqlit struct {
	driverName string
}

type OperatorBundle struct {
	name       string
	bundlepath string
	version    string
}

// NewSqlit creates a new sqlit instance.
func NewSqlit() *Sqlit {
	return &Sqlit{
		driverName: "sqlite3",
	}
}

// QueryOperatorBundle executes an sqlit query as an ordinary user and returns the result.
func (c *Sqlit) Query(dbFilePath string, query string) (*sql.Rows, error) {
	database, err := sql.Open(c.driverName, dbFilePath)
	defer database.Close()
	if err != nil {
		return nil, err
	}
	rows, err := database.Query(query)
	if err != nil {
		return nil, err
	}
	return rows, err
}

// QueryOperatorBundle executes an sqlit query as an ordinary user and returns the result.
func (c *Sqlit) QueryOperatorBundle(dbFilePath string) ([]OperatorBundle, error) {
	rows, err := c.Query(dbFilePath, "SELECT name,bundlepath,version FROM operatorbundle")
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	var OperatorBundles []OperatorBundle
	var name string
	var bundlepath string
	var version string
	for rows.Next() {
		rows.Scan(&name, &bundlepath, &version)
		OperatorBundles = append(OperatorBundles, OperatorBundle{name: name, bundlepath: bundlepath, version: version})
		e2e.Logf("OperatorBundles: name: %s,bundlepath: %s, version: %s", name, bundlepath, version)
	}
	return OperatorBundles, nil
}

// CheckOperatorBundlePathExist is to check the OperatorBundlePath exist
func (c *Sqlit) CheckOperatorBundlePathExist(dbFilePath string, bundlepath string) (bool, error) {
	OperatorBundles, err := c.QueryOperatorBundle(dbFilePath)
	if err != nil {
		return false, err
	}
	for _, OperatorBundle := range OperatorBundles {
		if strings.Compare(OperatorBundle.bundlepath, bundlepath) == 0 {
			return true, nil
		}
	}
	return false, nil
}

// CheckOperatorBundleNameExist is to check the OperatorBundleName exist
func (c *Sqlit) CheckOperatorBundleNameExist(dbFilePath string, bundleName string) (bool, error) {
	OperatorBundles, err := c.QueryOperatorBundle(dbFilePath)
	if err != nil {
		return false, err
	}
	for _, OperatorBundle := range OperatorBundles {
		if strings.Compare(OperatorBundle.name, bundleName) == 0 {
			return true, nil
		}
	}
	return false, nil
}
