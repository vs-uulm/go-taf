import { ref, Ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

/*
useTimeQuery keeps a selected time (in nanoseconds, undefined for the latest state) in the query parameter at, like the
search and filters of FacetedSearch, so that it is restored when returning to the page, e.g., with the back button. The
parameter is replaced instead of pushed, so that moving a slider does not add an entry to the history for every
position.
*/
export function useTimeQuery(): Ref<number | undefined> {
  const route = useRoute();
  const router = useRouter();

  const raw = route.query.at;
  const initial = Number(raw);
  const time = ref<number | undefined>(raw !== undefined && raw !== '' && Number.isFinite(initial) ? initial : undefined);

  watch(time, (value) => {
    const at = value === undefined ? undefined : String(Math.round(value));
    if (at === route.query.at) {
      return;
    }
    const query = { ...route.query };
    if (at === undefined) {
      delete query.at;
    } else {
      query.at = at;
    }
    router.replace({ query });
  });

  return time;
}
