import * as vlens from "vlens";
import * as server from "../server";

interface TagCacheState {
  tags: server.Tag[];
  loaded: boolean;
  loading: boolean;
}

const tagCacheState = vlens.declareHook(
  (): TagCacheState => ({
    tags: [],
    loaded: false,
    loading: false,
  })
);

export const useTagCache = () => {
  const state = tagCacheState();

  return {
    tags: state.tags,
    loaded: state.loaded,

    loadTags() {
      if (state.loaded || state.loading) return;
      state.loading = true;
      vlens.scheduleRedraw();

      server.ListTags({}).then(([result, error]) => {
        if (result && !error) {
          state.tags = result.tags;
        }
        state.loaded = true;
        state.loading = false;
        vlens.scheduleRedraw();
      });
    },

    getTag(id: number): server.Tag | undefined {
      return state.tags.find(t => t.id === id);
    },
  };
};
