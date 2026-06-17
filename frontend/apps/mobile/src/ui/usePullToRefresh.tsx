import { color } from '@swp/design-tokens';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import { RefreshControl } from 'react-native';

// Pull-to-refresh wired to TanStack Query. Refetches every query currently mounted
// on the screen (`type: 'active'`), so one pull refreshes exactly what the screen
// shows — no per-screen query-key bookkeeping. Pass `extra` for any non-query
// refresh the screen also needs (e.g. re-reading GPS).
//
// Returns a ready `<RefreshControl>` element to hand to a ScrollView/FlatList's
// `refreshControl` prop — React Native clones that element, so the hook must return
// an actual RefreshControl (not a wrapper component).
//
//   const refreshControl = usePullToRefresh();
//   <ScrollView refreshControl={refreshControl}>…</ScrollView>
export function usePullToRefresh(extra?: () => unknown) {
  const qc = useQueryClient();
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await Promise.all([qc.refetchQueries({ type: 'active' }), extra?.()]);
    } finally {
      setRefreshing(false);
    }
  }, [qc, extra]);

  return (
    <RefreshControl
      refreshing={refreshing}
      onRefresh={onRefresh}
      tintColor={color.primary}
      colors={[color.primary]}
    />
  );
}
