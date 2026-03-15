import { beforeAll, describe, expect, mock, test } from 'bun:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const identityT = (value) => value;

mock.module('@douyinfe/semi-ui', () => {
  const passthrough = ({ children }) => <div>{children}</div>;

  return {
    Button: ({ children }) => <button>{children}</button>,
    Modal: { confirm: () => {} },
    Space: passthrough,
    Tag: ({ children }) => <span>{children}</span>,
    Typography: {
      Text: ({ children, ...props }) => <span {...props}>{children}</span>,
    },
    Popover: passthrough,
    Divider: () => <hr />,
    Badge: () => <span />,
    Tooltip: passthrough,
  };
});

mock.module('../../../helpers', () => ({
  renderQuota: (value) => String(value),
}));

mock.module('../../../helpers/render', () => ({
  convertUSDToCurrency: (value) => `$${Number(value || 0).toFixed(2)}`,
}));

let getSubscriptionsColumns;

beforeAll(async () => {
  ({ getSubscriptionsColumns } = await import('./SubscriptionsColumnDefs'));
});

function renderPaymentColumn(record, enableEpay = false) {
  const columns = getSubscriptionsColumns({
    t: identityT,
    openEdit: () => {},
    setPlanEnabled: () => {},
    enableEpay,
  });
  const paymentColumn = columns.find((column) => column.title === '支付渠道');
  if (!paymentColumn) {
    throw new Error('payment column not found');
  }
  return renderToStaticMarkup(paymentColumn.render(undefined, record));
}

describe('getSubscriptionsColumns payment column', () => {
  test('renders purchase link when external checkout URL is configured', () => {
    const markup = renderPaymentColumn({
      plan: {
        purchase_link: 'https://billing.example.com/plan/basic',
      },
    });

    expect(markup).toContain('https://billing.example.com/plan/basic');
  });

  test('renders explicit empty state when no purchase method is configured', () => {
    const markup = renderPaymentColumn({
      plan: {},
    });

    expect(markup).toContain('未配置');
  });
});
