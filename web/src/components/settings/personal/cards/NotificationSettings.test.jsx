import { beforeAll, beforeEach, describe, expect, mock, test } from 'bun:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const capturedSwitchProps = [];
const identityT = (value) => value;

mock.module('@douyinfe/semi-ui', () => {
  const passthrough = ({ children }) => <div>{children}</div>;

  const Form = ({ children, getFormApi }) => {
    if (getFormApi) {
      getFormApi({
        setValues: () => {},
        validate: () => Promise.resolve(),
      });
    }
    return <form>{typeof children === 'function' ? children() : children}</form>;
  };

  Form.RadioGroup = passthrough;
  Form.AutoComplete = () => <div />;
  Form.Input = () => <div />;
  Form.Slot = passthrough;
  Form.Switch = (props) => {
    capturedSwitchProps.push(props);
    return <div>{props.label}</div>;
  };

  return {
    Avatar: passthrough,
    Button: ({ children }) => <button>{children}</button>,
    Card: ({ children, footer }) => (
      <section>
        {children}
        {footer}
      </section>
    ),
    Col: passthrough,
    Form,
    Radio: ({ children }) => <div>{children}</div>,
    Row: passthrough,
    Switch: () => <div />,
    TabPane: passthrough,
    Tabs: passthrough,
    Toast: {
      error: () => {},
    },
    Typography: {
      Text: ({ children }) => <span>{children}</span>,
    },
  };
});

mock.module('@douyinfe/semi-icons', () => ({
  IconBell: () => <span />,
  IconKey: () => <span />,
  IconLink: () => <span />,
  IconMail: () => <span />,
}));

mock.module('lucide-react', () => ({
  Bell: () => <span />,
  DollarSign: () => <span />,
  Settings: () => <span />,
  ShieldCheck: () => <span />,
}));

mock.module('../../../../helpers', () => ({
  API: {
    get: async () => ({ data: { success: true, data: {} } }),
    put: async () => ({ data: { success: true } }),
  },
  renderQuotaWithPrompt: (value) => String(value),
  showError: () => {},
  showSuccess: () => {},
}));

mock.module('../../../playground/CodeViewer', () => ({
  default: () => null,
}));

mock.module('../../../../context/Status', () => ({
  StatusContext: React.createContext([{ status: {} }]),
}));

mock.module('../../../../context/User', () => ({
  UserContext: React.createContext([{ user: { role: 0 } }]),
}));

mock.module('../../../../hooks/common/useUserPermissions', () => ({
  useUserPermissions: () => ({
    permissions: null,
    loading: false,
    hasSidebarSettingsPermission: () => false,
    isSidebarSectionAllowed: () => false,
    isSidebarModuleAllowed: () => false,
  }),
}));

mock.module('../../../../hooks/common/useSidebar', () => ({
  mergeAdminConfig: () => null,
  useSidebar: () => ({
    refreshUserConfig: async () => {},
  }),
}));

let NotificationSettings;

beforeAll(async () => {
  ({ default: NotificationSettings } = await import('./NotificationSettings'));
});

beforeEach(() => {
  capturedSwitchProps.length = 0;
});

describe('NotificationSettings', () => {
  test('keeps record ip switch checked and disabled', () => {
    renderToStaticMarkup(
      <NotificationSettings
        t={identityT}
        notificationSettings={{
          warningType: 'email',
          warningThreshold: 500000,
          webhookUrl: '',
          webhookSecret: '',
          notificationEmail: '',
          barkUrl: '',
          gotifyUrl: '',
          gotifyToken: '',
          gotifyPriority: 5,
          upstreamModelUpdateNotifyEnabled: false,
          acceptUnsetModelRatioModel: false,
          recordIpLog: false,
        }}
        handleNotificationSettingChange={() => {}}
        saveNotificationSettings={() => {}}
      />,
    );

    const recordIpSwitch = capturedSwitchProps.find(
      (props) => props.field === 'recordIpLog',
    );

    expect(recordIpSwitch).toBeDefined();
    expect(recordIpSwitch.checked).toBe(true);
    expect(recordIpSwitch.disabled).toBe(true);
  });
});
