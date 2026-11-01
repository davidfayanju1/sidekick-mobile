import type { UseQueryResult } from '@tanstack/react-query';
import * as React from 'react';
import Animated, { FadeIn } from 'react-native-reanimated';

import { ErrorState } from '@/components/UI/EmptyState';
import { errorMessage } from '@/lib/axios';

export interface QueryBoundaryProps<T> {
  query: UseQueryResult<T>;
  /** Shown on first load. Use skeletons shaped like the real content. */
  skeleton: React.ReactNode;
  /** Decides whether successful data counts as nothing to show. */
  isEmpty?: (data: T) => boolean;
  empty?: React.ReactNode;
  children: (data: T) => React.ReactNode;
}

/**
 * The four states every remote screen has, in one place.
 *
 * Only the *first* load shows the skeleton. A background refetch keeps the old
 * data on screen — flashing a skeleton over content the user is already reading
 * is the most common way a well-intentioned loading state makes an app feel
 * worse than no loading state at all.
 */
export function QueryBoundary<T>({
  query,
  skeleton,
  isEmpty,
  empty,
  children,
}: QueryBoundaryProps<T>) {
  if (query.isPending) return <>{skeleton}</>;

  if (query.isError) {
    return (
      <ErrorState
        body={errorMessage(query.error)}
        onRetry={() => query.refetch()}
        retrying={query.isFetching}
      />
    );
  }

  if (empty && isEmpty?.(query.data)) return <>{empty}</>;

  return (
    <Animated.View entering={FadeIn.duration(220)} style={{ flex: 1 }}>
      {children(query.data)}
    </Animated.View>
  );
}

export default QueryBoundary;
