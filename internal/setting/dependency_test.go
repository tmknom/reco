package setting

import (
	"reflect"
	"testing"
)

// TestDependencies は、設定の依存関係が YAML 設定仕様の表の並びで、依存する設定の無効の値を伴うことを確かめる。
func TestDependencies(t *testing.T) {
	want := []Dependency{
		{Prerequisite: DependabotAlerts, Dependent: DependabotSecurityUpdates, DisabledDependent: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesDisabled)},
		{Prerequisite: SecretScanningEnabled, Dependent: RepositoryPushProtection, DisabledDependent: BoolValue(false)},
	}
	if got := Dependencies(); !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies() = %+v, want %+v", got, want)
	}
}
