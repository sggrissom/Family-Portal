import * as vlens from "vlens";
import * as server from "../server";
import { filterQuery, parseFilterQuery, serverFilters } from "../lib/photoFilterQuery";

export interface PhotoFilterState {
  selectedPeopleIds: number[];
  selectedTagIds: number[];
  dateFrom: string;
  dateTo: string;
  isFilterPanelOpen: boolean;
  people: server.Person[];
  peopleLoaded: boolean;
  peopleLoading: boolean;
  tags: server.Tag[];
  tagsLoaded: boolean;
  tagsLoading: boolean;
}

interface NormalizedDateRange {
  from: Date | null;
  to: Date | null;
}

const createInitialState = (): PhotoFilterState => ({
  ...parseFilterQuery(BROWSER ? window.location.search : ""),
  isFilterPanelOpen: false,
  people: [],
  peopleLoaded: false,
  peopleLoading: false,
  tags: [],
  tagsLoaded: false,
  tagsLoading: false,
});

const photoFilterState = vlens.declareHook((): PhotoFilterState => createInitialState());

// Keeping the filter in the URL means leaving the grid and coming back, by the
// back button or the viewer's back link, lands on the same filtered grid.
const syncUrl = (state: PhotoFilterState) => {
  const route = window.location.pathname + filterQuery(state);
  history.replaceState(history.state, "", route);
};

export const usePhotoFilter = () => {
  const state = photoFilterState();

  const changed = () => {
    syncUrl(state);
    vlens.scheduleRedraw();
  };

  const togglePerson = (personId: number) => {
    const currentIndex = state.selectedPeopleIds.indexOf(personId);
    if (currentIndex === -1) {
      state.selectedPeopleIds = [...state.selectedPeopleIds, personId];
    } else {
      state.selectedPeopleIds = state.selectedPeopleIds.filter(id => id !== personId);
    }
    changed();
  };

  const setDateFrom = (date: string) => {
    state.dateFrom = date;
    changed();
  };

  const setDateTo = (date: string) => {
    state.dateTo = date;
    changed();
  };

  const toggleTag = (tagId: number) => {
    const currentIndex = state.selectedTagIds.indexOf(tagId);
    if (currentIndex === -1) {
      state.selectedTagIds = [...state.selectedTagIds, tagId];
    } else {
      state.selectedTagIds = state.selectedTagIds.filter(id => id !== tagId);
    }
    changed();
  };

  const loadTags = async () => {
    if (state.tagsLoaded || state.tagsLoading) {
      return;
    }

    state.tagsLoading = true;
    vlens.scheduleRedraw();

    try {
      const [result, error] = await server.ListTags({});
      if (result && !error) {
        state.tags = result.tags || [];
        state.tagsLoaded = true;
      }
    } catch (error) {
      console.error("Failed to load tags:", error);
    } finally {
      state.tagsLoading = false;
      vlens.scheduleRedraw();
    }
  };

  const clearAllFilters = () => {
    state.selectedPeopleIds = [];
    state.selectedTagIds = [];
    state.dateFrom = "";
    state.dateTo = "";
    changed();
  };

  const loadPeople = async () => {
    if (state.peopleLoaded || state.peopleLoading) {
      return;
    }

    state.peopleLoading = true;
    vlens.scheduleRedraw();

    try {
      const [result, error] = await server.ListPeople({});
      if (result && !error) {
        state.people = result.people || [];
        state.peopleLoaded = true;
      }
    } catch (error) {
      console.error("Failed to load people:", error);
    } finally {
      state.peopleLoading = false;
      vlens.scheduleRedraw();
    }
  };

  const toggleFilterPanel = async () => {
    state.isFilterPanelOpen = !state.isFilterPanelOpen;

    if (state.isFilterPanelOpen) {
      if (!state.peopleLoaded) {
        await loadPeople();
      }
      if (!state.tagsLoaded) {
        await loadTags();
      }
    }

    vlens.scheduleRedraw();
  };

  const hasActiveFilters = (): boolean => {
    return (
      state.selectedPeopleIds.length > 0 ||
      state.selectedTagIds.length > 0 ||
      !!state.dateFrom ||
      !!state.dateTo
    );
  };

  const getFilterSummary = (): string => {
    const parts = [];

    if (state.selectedPeopleIds.length > 0) {
      parts.push(`${state.selectedPeopleIds.length} people`);
    }

    if (state.selectedTagIds.length > 0) {
      parts.push(`${state.selectedTagIds.length} tags`);
    }

    if (state.dateFrom || state.dateTo) {
      const { from, to } = normalizeDateRange(state.dateFrom, state.dateTo);

      if (from && to) {
        parts.push(`${formatDateShort(from.toISOString())} - ${formatDateShort(to.toISOString())}`);
      } else if (from) {
        parts.push(`from ${formatDateShort(from.toISOString())}`);
      } else if (to) {
        parts.push(`until ${formatDateShort(to.toISOString())}`);
      }
    }

    return parts.join(", ");
  };

  return {
    serverFilters: () => serverFilters(state),
    selectedPeopleIds: state.selectedPeopleIds,
    selectedTagIds: state.selectedTagIds,
    dateFrom: state.dateFrom,
    dateTo: state.dateTo,
    isFilterPanelOpen: state.isFilterPanelOpen,
    people: state.people,
    peopleLoaded: state.peopleLoaded,
    peopleLoading: state.peopleLoading,
    tags: state.tags,
    tagsLoaded: state.tagsLoaded,
    tagsLoading: state.tagsLoading,
    togglePerson,
    toggleTag,
    setDateFrom,
    setDateTo,
    clearAllFilters,
    toggleFilterPanel,
    loadPeople,
    loadTags,
    hasActiveFilters,
    getFilterSummary,
  };
};

const formatDateShort = (dateString: string): string => {
  try {
    const date = new Date(dateString);
    return date.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: date.getFullYear() !== new Date().getFullYear() ? "numeric" : undefined,
    });
  } catch {
    return dateString;
  }
};

const normalizeDateRange = (rawFrom: string, rawTo: string): NormalizedDateRange => {
  const fromDate = parseDateOnly(rawFrom);
  const toDate = parseDateOnly(rawTo);

  if (!fromDate || !toDate) {
    return {
      from: fromDate,
      to: toDate ? asEndOfDay(toDate) : null,
    };
  }

  if (fromDate <= toDate) {
    return {
      from: fromDate,
      to: asEndOfDay(toDate),
    };
  }

  return {
    from: toDate,
    to: asEndOfDay(fromDate),
  };
};

const parseDateOnly = (value: string): Date | null => {
  if (!value) {
    return null;
  }

  const date = new Date(`${value}T00:00:00`);
  if (isNaN(date.getTime())) {
    return null;
  }

  return date;
};

const asEndOfDay = (date: Date): Date => {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59, 999);
};
