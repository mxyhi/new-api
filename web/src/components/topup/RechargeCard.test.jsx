import { beforeAll, describe, expect, mock, test } from 'bun:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const identityT = (value) => value;

mock.module('@douyinfe/semi-ui', () => {
  const passthrough = ({ children }) => <div>{children}</div>;
  const formComponent = ({ children }) => <form>{children}</form>;

  formComponent.InputNumber = () => <input />;
  formComponent.Slot = ({ children }) => <div>{children}</div>;

  return {
    Avatar: passthrough,
    Banner: ({ description }) => <div>{description}</div>,
    Button: ({ children }) => <button>{children}</button>,
    Card: ({ children }) => <section>{children}</section>,
    Col: passthrough,
    Form: formComponent,
    Row: passthrough,
    Skeleton: {
      Title: () => <div />,
    },
    Space: passthrough,
    Spin: () => <div>loading</div>,
    TabPane: ({ children }) => <div>{children}</div>,
    Tag: ({ children }) => <span>{children}</span>,
    Tabs: ({ children }) => <div>{children}</div>,
    Tooltip: passthrough,
    Typography: {
      Text: ({ children }) => <span>{children}</span>,
    },
  };
});

mock.module('react-icons/si', () => ({
  SiAlipay: () => <span />,
  SiStripe: () => <span />,
  SiWechat: () => <span />,
}));

mock.module('lucide-react', () => ({
  BarChart2: () => <span />,
  Coins: () => <span />,
  CreditCard: () => <span />,
  Receipt: () => <span />,
  Sparkles: () => <span />,
  TrendingUp: () => <span />,
  Wallet: () => <span />,
}));

mock.module('../../hooks/common/useMinimumLoadingTime', () => ({
  useMinimumLoadingTime: () => false,
}));

mock.module('../../helpers/render', () => ({
  getCurrencyConfig: () => ({ symbol: '$', rate: 1, type: 'USD' }),
}));

mock.module('./SubscriptionPlansCard', () => ({
  default: ({ redemptionSlot }) => (
    <div>
      <div>我的订阅</div>
      {redemptionSlot}
      <div>套餐列表</div>
    </div>
  ),
}));

mock.module('./RedemptionCard', () => ({
  default: () => <div>兑换码卡片</div>,
}));

let RechargeCard;

beforeAll(async () => {
  ({ default: RechargeCard } = await import('./RechargeCard'));
});

describe('RechargeCard', () => {
  test('keeps redemption card in topup tab and subscription section when plans exist', () => {
    const markup = renderToStaticMarkup(
      <RechargeCard
        t={identityT}
        enableOnlineTopUp={false}
        enableStripeTopUp={false}
        enableCreemTopUp={false}
        creemProducts={[]}
        creemPreTopUp={() => {}}
        presetAmounts={[]}
        selectedPreset={null}
        selectPresetAmount={() => {}}
        formatLargeNumber={(value) => String(value)}
        priceRatio={1}
        topUpCount={1}
        minTopUp={1}
        renderQuotaWithAmount={(value) => String(value)}
        getAmount={async () => {}}
        setTopUpCount={() => {}}
        setSelectedPreset={() => {}}
        renderAmount={() => '$0.00'}
        amountLoading={false}
        payMethods={[]}
        preTopUp={() => {}}
        paymentLoading={false}
        payWay=''
        redemptionCode=''
        setRedemptionCode={() => {}}
        topUp={() => {}}
        isSubmitting={false}
        topUpLink=''
        openTopUpLink={() => {}}
        userState={{ user: { quota: 0, used_quota: 0, request_count: 0 } }}
        renderQuota={(value) => String(value)}
        statusLoading={false}
        topupInfo={{ amount_options: [], discount: {} }}
        onOpenHistory={() => {}}
        subscriptionLoading={false}
        subscriptionPlans={[{ plan: { id: 1, title: '专业套餐' } }]}
        billingPreference='wallet_first'
        onChangeBillingPreference={() => {}}
        activeSubscriptions={[]}
        allSubscriptions={[]}
        reloadSubscriptionSelf={async () => {}}
      />,
    );

    expect(markup.match(/兑换码卡片/g)?.length ?? 0).toBe(2);
  });
});
