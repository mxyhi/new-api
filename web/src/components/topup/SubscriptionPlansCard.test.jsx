import { beforeAll, describe, expect, mock, test } from 'bun:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const identityT = (value) => value;

mock.module('@douyinfe/semi-ui', () => {
  const passthrough = ({ children }) => <div>{children}</div>;

  return {
    Badge: () => <span />,
    Button: ({ children }) => <button>{children}</button>,
    Card: ({ children }) => <section>{children}</section>,
    Divider: () => <hr />,
    Select: ({ value }) => <select value={value} readOnly />,
    Skeleton: {
      Title: () => <div />,
      Paragraph: () => <div />,
      Button: () => <div />,
    },
    Space: passthrough,
    Tag: ({ children }) => <span>{children}</span>,
    Tooltip: passthrough,
    Typography: {
      Text: ({ children }) => <span>{children}</span>,
      Title: ({ children }) => <h3>{children}</h3>,
    },
  };
});

mock.module('../../helpers', () => ({
  API: {
    post: async () => ({ data: { message: 'success', data: {} } }),
  },
  showError: () => {},
  showSuccess: () => {},
  renderQuota: (value) => String(value),
}));

mock.module('../../helpers/render', () => ({
  getCurrencyConfig: () => ({ symbol: '$', rate: 1, type: 'USD' }),
}));

mock.module('../../helpers/subscriptionFormat', () => ({
  formatSubscriptionDuration: () => '30天',
  formatSubscriptionResetPeriod: () => '每月',
}));

mock.module('lucide-react', () => ({
  RefreshCw: () => <span />,
  Sparkles: () => <span />,
}));

mock.module('./modals/SubscriptionPurchaseModal', () => ({
  default: () => null,
}));

let SubscriptionPlansCard;

beforeAll(async () => {
  ({ default: SubscriptionPlansCard } = await import('./SubscriptionPlansCard'));
});

describe('SubscriptionPlansCard', () => {
  test('renders redemption slot between subscription summary and plan list', () => {
    const markup = renderToStaticMarkup(
      <SubscriptionPlansCard
        t={identityT}
        plans={[
          {
            plan: {
              id: 1,
              title: '专业套餐',
              price_amount: 9.9,
              total_amount: 1000,
            },
          },
        ]}
        payMethods={[]}
        activeSubscriptions={[]}
        allSubscriptions={[]}
        billingPreference='wallet_first'
        onChangeBillingPreference={() => {}}
        reloadSubscriptionSelf={async () => {}}
        redemptionSlot={<div>兑换码入口</div>}
        withCard={false}
      />,
    );

    expect(markup).toContain('我的订阅');
    expect(markup).toContain('兑换码入口');
    expect(markup).toContain('专业套餐');
    expect(markup.indexOf('我的订阅')).toBeLessThan(markup.indexOf('兑换码入口'));
    expect(markup.indexOf('兑换码入口')).toBeLessThan(markup.indexOf('专业套餐'));
  });
});
