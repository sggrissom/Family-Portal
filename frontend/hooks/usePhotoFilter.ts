import * as vlens from "vlens";
import * as server from "../server";

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
  selectedPeopleIds: [],
  selectedTagIds: [],
  dateFrom: "",
  dateTo: "",
  isFilterPanelOpen: false,
  people: [],
  peopleLoaded: false,
  peopleLoading: false,
  tags: [],
  tagsLoaded: false,
  tagsLoading: false,
});

const photoFilterState = vlens.declareHook((): PhotoFilterState => createInitialState());

export const usePhotoFilter = () => {
  const state = photoFilterState();

  const togglePerson = (personId: number) => {
    const currentIndex = state.selectedPeopleIds.indexOf(personId);
    if (currentIndex === -1) {
      // Add person
      state.selectedPeopleIds = [...state.selectedPeopleIds, personId];
    } else {
      // Remove person
      state.selectedPeopleIds = state.selectedPeopleIds.filter(id => id !== personId);
    }
    vlens.scheduleRedraw();
  };

  const setDateFrom = (date: string) => {
    state.dateFrom = date;
    vlens.scheduleRedraw();
  };

  const setDateTo = (date: string) => {
    state.dateTo = date;
    vlens.scheduleRedraw();
  };

  const toggleTag = (tagId: number) => {
    const currentIndex = state.selectedTagIds.indexOf(tagId);
    if (currentIndex === -1) {
      state.selectedTagIds = [...state.selectedTagIds, tagId];
    } else {
      state.selectedTagIds = state.selectedTagIds.filter(id => id !== tagId);
    }
    vlens.scheduleRedraw();
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
    vlens.scheduleRedraw();
  };

  const loadPeople = async () => {
    if (state.peopleLoaded || state.peopleLoading) {
      return; // Already loaded or loading
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

  const filterPhotos = (photos: server.PhotoWithPeople[]): server.PhotoWithPeople[] => {
    let filtered = photos;

    // Filter by people
    if (state.selectedPeopleIds.length > 0) {
      filtered = filtered.filter(photoWithPeople => {
        // Show if ANY selected person is in the photo
        return state.selectedPeopleIds.some(selectedId =>
          photoWithPeople.people.some(person => person.id === selectedId)
        );
      });
    }

    // Filter by tags
    if (state.selectedTagIds.length > 0) {
      filtered = filtered.filter(p =>
        state.selectedTagIds.some(tagId => p.image.tagIds?.includes(tagId))
      );
    }

    // Filter by date range
    if (state.dateFrom || state.dateTo) {
      const { from, to } = normalizeDateRange(state.dateFrom, state.dateTo);

      filtered = filtered.filter(photoWithPeople => {
        const photoDate = photoWithPeople.image.photoDate;
        if (!photoDate) return false;

        const date = new Date(photoDate);

        if (from && date < from) return false;
        if (to && date > to) return false;

        return true;
      });
    }

    return filtered;
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
    filterPhotos,
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
