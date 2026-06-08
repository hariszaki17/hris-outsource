// Smoke screen — proves the three surface-agnostic shared packages resolve and run in RN.
// NOT a feature screen (feature screens land in later milestones).
import { ApiError } from '@swp/api-client';
import { color } from '@swp/design-tokens';
import { TZ, formatLocalTime } from '@swp/shared/datetime';
import { ID_PREFIX, asId, prefixOf } from '@swp/shared/ids';
import { View } from 'react-native';
import { Screen } from '../src/ui/Screen';
import { Text } from '../src/ui/Text';

export default function Index() {
  // @swp/shared/ids — branded ID helpers.
  const employeeId = asId(ID_PREFIX.EMPLOYEE, 'SWP-EMP-1042');
  const prefix = prefixOf(employeeId);
  // @swp/shared/datetime — Asia/Jakarta TZ layer.
  const shiftStart = formatLocalTime('09:00');
  // @swp/api-client — value import proves bundle resolution.
  const apiErrorName = ApiError.name;

  return (
    <Screen>
      <Text variant="title">SWP HRIS — Mobile</Text>
      <Text variant="caption" className="mb-6">
        Expo scaffold · shared packages wired
      </Text>

      <View className="rounded-lg bg-surface p-4">
        <Text variant="caption">@swp/shared/ids</Text>
        <Text variant="body">
          {employeeId} (prefix {prefix})
        </Text>

        <Text variant="caption" className="mt-3">
          @swp/shared/datetime
        </Text>
        <Text variant="body">
          shift start {shiftStart} · {TZ}
        </Text>

        <Text variant="caption" className="mt-3">
          @swp/api-client
        </Text>
        <Text variant="body">error class: {apiErrorName}</Text>

        <Text variant="caption" className="mt-3">
          @swp/design-tokens
        </Text>
        <Text variant="body" style={{ color: color.primary }}>
          primary {color.primary}
        </Text>
      </View>
    </Screen>
  );
}
