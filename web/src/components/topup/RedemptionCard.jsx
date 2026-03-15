/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useRef } from 'react';
import { Button, Card, Form, Typography } from '@douyinfe/semi-ui';
import { IconGift } from '@douyinfe/semi-icons';

const { Text } = Typography;

const RedemptionCard = ({
  t,
  redemptionCode,
  setRedemptionCode,
  topUp,
  isSubmitting,
  topUpLink,
  openTopUpLink,
}) => {
  const redeemFormApiRef = useRef(null);

  useEffect(() => {
    const formApi = redeemFormApiRef.current;
    if (!formApi) return;

    const nextRedemptionCode = redemptionCode ?? '';
    if (formApi.getValue('redemptionCode') !== nextRedemptionCode) {
      // Semi Form keeps its own field state, so external resets need to sync back into the input.
      formApi.setValue('redemptionCode', nextRedemptionCode);
    }
  }, [redemptionCode]);

  return (
    <Card
      className='!rounded-xl w-full'
      title={
        <Text type='tertiary' strong>
          {t('兑换码')}
        </Text>
      }
    >
      <Form
        getFormApi={(api) => (redeemFormApiRef.current = api)}
        initValues={{ redemptionCode: redemptionCode }}
      >
        <Form.Input
          field='redemptionCode'
          noLabel={true}
          placeholder={t('请输入兑换码')}
          value={redemptionCode}
          onChange={(value) => setRedemptionCode(value)}
          prefix={<IconGift />}
          suffix={
            <div className='flex items-center gap-2'>
              <Button
                type='primary'
                theme='solid'
                onClick={topUp}
                loading={isSubmitting}
              >
                {t('立即兑换')}
              </Button>
            </div>
          }
          showClear
          style={{ width: '100%' }}
          extraText={
            <div>
              <Text type='tertiary'>
                {t('兑换码可用于充值额度或激活订阅套餐')}
              </Text>
              {topUpLink && (
                <Text type='tertiary'>
                  {' '}
                  {t('在找兑换码？')}
                  <Text
                    type='secondary'
                    underline
                    className='cursor-pointer'
                    onClick={openTopUpLink}
                  >
                    {t('购买兑换码')}
                  </Text>
                </Text>
              )}
            </div>
          }
        />
      </Form>
    </Card>
  );
};

export default RedemptionCard;
