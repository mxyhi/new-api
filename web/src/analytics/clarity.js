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

import Clarity from '@microsoft/clarity';

const clarityInitFlagKey = '__NEW_API_CLARITY_INITIALIZED__';

const getClarityProjectId = () => {
  if (typeof window === 'undefined') {
    return '';
  }

  const runtimeConfig = window.__NEW_API_RUNTIME_CONFIG;
  if (!runtimeConfig || typeof runtimeConfig.clarityProjectId !== 'string') {
    return '';
  }

  return runtimeConfig.clarityProjectId.trim();
};

export const initClarity = () => {
  if (typeof window === 'undefined' || window[clarityInitFlagKey]) {
    return;
  }

  const projectId = getClarityProjectId();
  if (!projectId) {
    return;
  }

  Clarity.init(projectId);
  window[clarityInitFlagKey] = true;
};
