package settingsmessaging

import (
	"errors"
	"reflect"
	"testing"

	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestNotificationCenterFeishuPatchKeepsSecretAndNormalizesRecipients(t *testing.T) {
	current := NotificationCenterDefaultSetting()
	current.Channels.Feishu.AppSecret = "saved-secret"
	enabled := true
	appID := "  cli_demo  "
	emptySecret := "  "
	receiveIDType := " OPEN_ID "
	recipients := []string{"  ou_first ", "ou_first", "ou_second"}

	next, err := ApplyNotificationCenterSettingPatch(current, NotificationCenterSettingPatch{
		Channels: &NotificationChannelsPatch{
			Feishu: &FeishuNotificationChannelPatch{
				Enabled:       &enabled,
				AppID:         &appID,
				AppSecret:     &emptySecret,
				ReceiveIDType: &receiveIDType,
				Recipients:    &recipients,
			},
		},
	})
	if err != nil {
		t.Fatalf("apply feishu patch: %v", err)
	}
	if next.Channels.Feishu.AppID != "cli_demo" {
		t.Fatalf("unexpected app id: %q", next.Channels.Feishu.AppID)
	}
	if next.Channels.Feishu.AppSecret != "saved-secret" {
		t.Fatalf("empty patch must preserve secret, got %q", next.Channels.Feishu.AppSecret)
	}
	if next.Channels.Feishu.ReceiveIDType != FeishuReceiveIDTypeOpenID {
		t.Fatalf("unexpected receive id type: %q", next.Channels.Feishu.ReceiveIDType)
	}
	if !reflect.DeepEqual(next.Channels.Feishu.Recipients, []string{"ou_first", "ou_second"}) {
		t.Fatalf("unexpected recipients: %#v", next.Channels.Feishu.Recipients)
	}
}

func TestNotificationCenterFeishuMaskAndPersistenceRoundTrip(t *testing.T) {
	setting := NotificationCenterDefaultSetting()
	setting.Channels.Feishu = FeishuNotificationChannelSetting{
		Enabled:       true,
		AppID:         "cli_demo",
		AppSecret:     "top-secret",
		ReceiveIDType: FeishuReceiveIDTypeChatID,
		Recipients:    []string{"oc_demo"},
	}

	encoded := jsonmap.JSON(NotificationCenterSettingToMap(setting))
	decoded := DecodeNotificationCenterSetting(encoded, NotificationCenterDefaultSetting())
	if decoded.Channels.Feishu.AppSecret != "top-secret" {
		t.Fatalf("persisted secret was not decoded: %q", decoded.Channels.Feishu.AppSecret)
	}
	if !reflect.DeepEqual(decoded.Channels.Feishu.Recipients, []string{"oc_demo"}) {
		t.Fatalf("unexpected decoded recipients: %#v", decoded.Channels.Feishu.Recipients)
	}

	masked := MaskNotificationCenterSettingForAdmin(setting)
	channels := settingsvalue.ToStringAnyMap(masked["channels"])
	feishu := settingsvalue.ToStringAnyMap(channels["feishu"])
	if feishu["app_secret"] != "" {
		t.Fatalf("admin response leaked app secret: %#v", feishu["app_secret"])
	}
	if feishu["has_app_secret"] != true {
		t.Fatalf("admin response must expose configured state: %#v", feishu["has_app_secret"])
	}
}

func TestValidateNotificationCenterFeishuConfiguration(t *testing.T) {
	valid := NotificationCenterDefaultSetting()
	valid.Channels.Feishu = FeishuNotificationChannelSetting{
		Enabled:       true,
		AppID:         "cli_demo",
		AppSecret:     "secret",
		ReceiveIDType: FeishuReceiveIDTypeChatID,
		Recipients:    []string{"oc_demo"},
	}
	if err := ValidateNotificationCenterSetting(valid); err != nil {
		t.Fatalf("expected valid feishu configuration, got %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*NotificationCenterSetting)
	}{
		{"missing app id", func(setting *NotificationCenterSetting) { setting.Channels.Feishu.AppID = "" }},
		{"missing app secret", func(setting *NotificationCenterSetting) { setting.Channels.Feishu.AppSecret = "" }},
		{"missing recipients", func(setting *NotificationCenterSetting) { setting.Channels.Feishu.Recipients = nil }},
		{"unsupported receive id type", func(setting *NotificationCenterSetting) { setting.Channels.Feishu.ReceiveIDType = "department_id" }},
		{"invalid recipient email", func(setting *NotificationCenterSetting) {
			setting.Channels.Feishu.ReceiveIDType = FeishuReceiveIDTypeEmail
			setting.Channels.Feishu.Recipients = []string{"not-an-email"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setting := valid
			test.mutate(&setting)
			if err := ValidateNotificationCenterSetting(setting); !errors.Is(err, ErrNotificationConfigInvalid) {
				t.Fatalf("expected invalid notification config, got %v", err)
			}
		})
	}
}
