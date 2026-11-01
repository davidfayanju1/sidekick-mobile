import { useInfiniteQuery } from '@tanstack/react-query';

import { qk } from '@/lib/api/keys';
import type { TaskCard } from '@/lib/api/schemas';
import { api } from '@/lib/axios';

export interface FeedFilters {
  lat?: number;
  lng?: number;
  radius_km?: number;
  category?: string;
  min_budget?: number;
  max_budget?: number;
  timing_type?: 'asap' | 'specific_date' | 'flexible_range';
  sort?: string;
  limit?: number;
}

interface FeedPage {
  tasks: TaskCard[];
  next_cursor: string | null;
  radius_km_applied?: number;
}

interface SearchPage {
  results: TaskCard[];
  next_cursor: string | null;
}

/**
 * The browse feed, geo-sorted and paginated by cursor.
 *
 * Infinite rather than paged: this is a swipe deck and an endless list, and
 * neither has anywhere sensible to put page numbers.
 */
export function useFeed(filters: FeedFilters = {}) {
  return useInfiniteQuery({
    queryKey: qk.discovery.feed(filters),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data } = await api.get<FeedPage>('/feed', {
        params: { ...filters, cursor: pageParam },
      });
      return data;
    },
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
    // A feed that reshuffles under the user's thumb mid-swipe is maddening.
    staleTime: 60_000,
  });
}

/** Flattens the pages — screens want one list, not a list of pages. */
export function flattenFeed(pages: FeedPage[] | undefined): TaskCard[] {
  return pages?.flatMap((page) => page.tasks ?? []) ?? [];
}

export function useSearch(query: string, filters: FeedFilters = {}) {
  return useInfiniteQuery({
    queryKey: qk.discovery.search(query, filters),
    // Nothing to search for until they've typed something worth matching.
    enabled: query.trim().length >= 2,
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data } = await api.get<SearchPage>('/search', {
        params: { q: query, ...filters, cursor: pageParam },
      });
      return data;
    },
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
  });
}

export function flattenSearch(pages: SearchPage[] | undefined): TaskCard[] {
  return pages?.flatMap((page) => page.results ?? []) ?? [];
}
