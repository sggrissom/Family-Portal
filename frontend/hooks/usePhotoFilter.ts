import * as vlens from "vlens";
import * as server from "../server";

export interface PhotoFilterState {
  selectedPeopleIds: number[];
  dateFrom: string;
  dateTo: string;
  isFilterPanelOpen: boolean;
  people: server.Person[];
  peopleLoaded: boolean;
  peopleLoading: boolean;
}

interface NormalizedDateRange {
  from: Date | null;
  to: Date | null;
}

const createInitialState = (): PhotoFilterState => ({
  selectedPeopleIds: [],
  dateFrom: "",
  dateTo: "",
  isFilterPanelOpen: false,
  people: [],
  peopleLoaded: false,
  peopleLoading: false,
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

  const clearAllFilters = () => {
    state.selectedPeopleIds = [];
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

    // Load people when opening the filter panel for the first time
    if (state.isFilterPanelOpen && !state.peopleLoaded) {
      await loadPeople();
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
    return state.selectedPeopleIds.length > 0 || !!state.dateFrom || !!state.dateTo;
  };

  const getFilterSummary = (): string => {
    const parts = [];

    if (state.selectedPeopleIds.length > 0) {
      parts.push(`${state.selectedPeopleIds.length} people`);
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
    dateFrom: state.dateFrom,
    dateTo: state.dateTo,
    isFilterPanelOpen: state.isFilterPanelOpen,
    people: state.people,
    peopleLoaded: state.peopleLoaded,
    peopleLoading: state.peopleLoading,
    togglePerson,
    setDateFrom,
    setDateTo,
    clearAllFilters,
    toggleFilterPanel,
    loadPeople,
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
  return new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
    23,
    59,
    59,
    999
  );
};
