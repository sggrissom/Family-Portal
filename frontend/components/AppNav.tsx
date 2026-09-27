import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../server";
import * as auth from "../lib/authCache";
import { copy } from "../lib/copy";
import { Destination, activeDestination, addPath, contextPersonId } from "../lib/appNav";
import { chipOrder } from "../lib/familyGroups";
import { dayKey, dayLabel } from "../lib/daySummary";
import { localDateString } from "../lib/when";
import { handOffPhotos, readLastPerson, rememberReturn, writeLastPerson } from "../lib/addFlow";
import {
  ModalDialogState,
  attrsModalDialog,
  closeModalDialog,
  newModalDialog,
} from "../lib/modalDialog";
import { PersonChips, initial, scrollSelectedChipIntoView } from "./PersonChips";
import "./app-nav-styles";

const DESTINATIONS: { key: Destination; href: string; icon: string }[] = [
  { key: "home", href: "/dashboard", icon: "🏠" },
  { key: "photos", href: "/photos", icon: "🖼️" },
  { key: "growth", href: "/growth", icon: "📈" },
  { key: "history", href: "/history", icon: "📅" },
  { key: "chat", href: "/chat", icon: "💬" },
];

const BOTTOM_BAR: Destination[] = ["home", "photos", "growth", "chat"];

interface NavState {
  accountOpen: boolean;
  sheetOpen: boolean;
  selectedPersonId: number | null;
  faceCount: number | null;
  dialog: ModalDialogState;
}

let familyPeople: server.Person[] | null = null;
let openEvents: server.OpenEvent[] = [];

const useNav = vlens.declareHook((): NavState => {
  const state: NavState = {
    accountOpen: false,
    sheetOpen: false,
    selectedPersonId: null,
    faceCount: null,
    dialog: newModalDialog(),
  };
  state.dialog.onDismiss = () => closeSheet(state);
  return state;
});

export const TopNav = ({ user }: { user: auth.AuthCache }) => {
  const state = useNav();
  const active = activeDestination(window.location.pathname);

  return (
    <>
      <ul className="app-topbar-links">
        {DESTINATIONS.map(d => (
          <li key={d.key}>
            <a
              href={d.href}
              className={d.key === active ? "active" : ""}
              aria-current={d.key === active ? "page" : undefined}
            >
              {copy.nav[d.key]}
            </a>
          </li>
        ))}
      </ul>
      <div className="app-topbar-actions">
        <button
          type="button"
          className="app-add-button"
          onClick={vlens.cachePartial(openSheet, state)}
        >
          <span aria-hidden="true">+</span> {copy.nav.add}
        </button>
        <div className="account">
          <button
            type="button"
            id="accountToggle"
            className={state.accountOpen ? "account-toggle open" : "account-toggle"}
            aria-label={copy.nav.account}
            aria-expanded={state.accountOpen ? "true" : "false"}
            aria-controls="accountMenu"
            onClick={vlens.cachePartial(toggleAccount, state)}
          >
            {initial(user.name)}
          </button>
          {state.accountOpen && <AccountMenu user={user} state={state} />}
        </div>
      </div>
    </>
  );
};

export const BottomNav = () => {
  const state = useNav();
  const active = activeDestination(window.location.pathname);
  const items = BOTTOM_BAR.map(key => DESTINATIONS.find(d => d.key === key)!);

  const link = (d: (typeof DESTINATIONS)[number]) => (
    <a
      key={d.key}
      href={d.href}
      className={d.key === active ? "app-bottombar-item active" : "app-bottombar-item"}
      aria-current={d.key === active ? "page" : undefined}
    >
      <span className="app-bottombar-icon" aria-hidden="true">
        {d.icon}
      </span>
      <span>{copy.nav[d.key]}</span>
    </a>
  );

  return (
    <nav className="app-bottombar" aria-label="Quick navigation">
      {items.slice(0, 2).map(link)}
      <button
        type="button"
        className="app-bottombar-add"
        aria-label={copy.nav.add}
        onClick={vlens.cachePartial(openSheet, state)}
      >
        <span aria-hidden="true">+</span>
      </button>
      {items.slice(2).map(link)}
    </nav>
  );
};

export const AddSheet = () => {
  const state = useNav();
  if (!state.sheetOpen) return null;

  const people = familyPeople ?? [];
  const personId = state.selectedPersonId;

  return (
    <div className="add-sheet-backdrop" onClick={vlens.cachePartial(backdropClicked, state)}>
      <div
        className="add-sheet"
        role="dialog"
        aria-modal="true"
        aria-labelledby="addSheetTitle"
        {...attrsModalDialog(state.dialog)}
      >
        <div className="add-sheet-header">
          <h2 id="addSheetTitle">{copy.addSheet.title}</h2>
          <button
            type="button"
            className="add-sheet-close"
            aria-label={copy.addSheet.close}
            onClick={vlens.cachePartial(closeSheet, state)}
          >
            ×
          </button>
        </div>

        {people.length > 0 && (
          <PersonChips
            people={people}
            selected={personId ? [personId] : []}
            onToggle={vlens.cachePartial(choosePerson, state)}
            label={copy.addSheet.whoFor}
          />
        )}

        <div className="add-sheet-options">
          <button type="button" onClick={pickPhotos}>
            <span className="add-sheet-icon" aria-hidden="true">
              📷
            </span>
            {copy.addSheet.photos}
          </button>
          <input
            id="addSheetPhotoInput"
            type="file"
            accept="image/*"
            multiple
            hidden
            onChange={vlens.cachePartial(photosPicked, state)}
          />
          <a
            href={addPath("/add-growth", personId)}
            onClick={vlens.cachePartial(formChosen, state)}
          >
            <span className="add-sheet-icon" aria-hidden="true">
              📏
            </span>
            {copy.addSheet.measurement}
          </a>
          <a
            href={addPath("/add-milestone", personId)}
            onClick={vlens.cachePartial(formChosen, state)}
          >
            <span className="add-sheet-icon" aria-hidden="true">
              ⭐
            </span>
            {copy.addSheet.milestone}
          </a>
          {openEvents.slice(0, 2).map(open => (
            <a
              key={open.event.id}
              href={`/competition/${open.event.id}`}
              onClick={vlens.cachePartial(formChosen, state)}
            >
              <span className="add-sheet-icon" aria-hidden="true">
                🏆
              </span>
              {copy.result} — {open.event.name}
              <span className="add-sheet-when">
                {dayLabel(dayKey(open.event.startDate), localDateString(new Date()))}
              </span>
            </a>
          ))}
        </div>
      </div>
    </div>
  );
};

const AccountMenu = ({ user, state }: { user: auth.AuthCache; state: NavState }) => (
  <div id="accountMenu" className="account-menu">
    <div className="account-menu-who">
      <span className="account-menu-eyebrow">{copy.account.signedInAs}</span>
      <strong>{user.name}</strong>
    </div>
    <ul>
      <li className="account-menu-history">
        <a href="/history">{copy.nav.history}</a>
      </li>
      <li>
        <a href="/activities">{copy.account.activities}</a>
      </li>
      <li>
        <a href="/manage-tags">{copy.account.tags}</a>
      </li>
      <li>
        <a href="/faces">
          {copy.account.faceReview}
          {state.faceCount ? <span className="count-badge">{state.faceCount}</span> : null}
        </a>
      </li>
      <li>
        <a href="/settings">{copy.account.settings}</a>
      </li>
      <li>
        <a href="/import">{copy.account.importExport}</a>
      </li>
      {user.isAdmin && (
        <li>
          <a href="/admin">{copy.account.admin}</a>
        </li>
      )}
    </ul>
    <button type="button" className="account-menu-logout" onClick={logoutClicked}>
      {copy.account.logOut}
    </button>
  </div>
);

function defaultPerson(people: server.Person[]): number | null {
  const candidates = [contextPersonId(window.location.pathname), readLastPerson()];
  return candidates.find(id => id !== null && people.some(p => p.id === id)) ?? null;
}

export function openAddSheet() {
  openSheet(useNav());
}

async function openSheet(state: NavState) {
  closeAccount(state);
  state.sheetOpen = true;
  state.selectedPersonId = defaultPerson(familyPeople ?? []);
  vlens.scheduleRedraw();

  const today = localDateString(new Date());
  const [[resp], [events]] = await Promise.all([
    server.ListPeople({}),
    server.ListOpenEvents({ today }),
  ]);
  openEvents = events?.events ?? [];
  if (!resp) return;
  familyPeople = chipOrder(resp.people || [], resp.relations || [], auth.getAuth()?.familyId ?? 0);
  if (state.sheetOpen) {
    state.selectedPersonId = defaultPerson(familyPeople);
  }
  vlens.scheduleRedraw();
  scrollSelectedChipIntoView();
}

function closeSheet(state: NavState) {
  if (!state.sheetOpen) return;
  state.sheetOpen = false;
  closeModalDialog(state.dialog);
  vlens.scheduleRedraw();
}

function backdropClicked(state: NavState, event: MouseEvent) {
  if (event.target === event.currentTarget) {
    closeSheet(state);
  }
}

function currentPath(): string {
  return window.location.pathname + window.location.search;
}

function formChosen(state: NavState) {
  rememberReturn(currentPath());
  closeSheet(state);
}

function pickPhotos() {
  document.getElementById("addSheetPhotoInput")?.click();
}

function photosPicked(state: NavState, event: Event) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = "";
  if (files.length === 0) return;
  handOffPhotos(files);
  rememberReturn(currentPath());
  const personId = state.selectedPersonId;
  closeSheet(state);
  core.setRoute(addPath("/add-photo", personId));
}

function choosePerson(state: NavState, personId: number) {
  state.selectedPersonId = state.selectedPersonId === personId ? null : personId;
  writeLastPerson(state.selectedPersonId);
  vlens.scheduleRedraw();
}

let detachAccountDismissal: (() => void) | null = null;
core.registerCleanupFunction(() => {
  detachAccountDismissal?.();
  detachAccountDismissal = null;
});

function closeAccount(state: NavState) {
  detachAccountDismissal?.();
  detachAccountDismissal = null;
  state.accountOpen = false;
  vlens.scheduleRedraw();
}

function toggleAccount(state: NavState) {
  if (state.accountOpen) {
    closeAccount(state);
    return;
  }

  state.accountOpen = true;
  vlens.scheduleRedraw();

  const handleClickOutside = (event: MouseEvent) => {
    const account = document.querySelector(".account");
    if (event.target instanceof Node && account && !account.contains(event.target)) {
      closeAccount(state);
    }
  };
  const handleEscape = (event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    closeAccount(state);
    document.getElementById("accountToggle")?.focus();
  };
  document.addEventListener("mousedown", handleClickOutside);
  document.addEventListener("keydown", handleEscape);
  detachAccountDismissal = () => {
    document.removeEventListener("mousedown", handleClickOutside);
    document.removeEventListener("keydown", handleEscape);
  };

  if (state.faceCount === null) {
    loadFaceCount(state);
  }
}

async function loadFaceCount(state: NavState) {
  state.faceCount = 0;
  const [resp] = await server.GetFaceReview({});
  if (!resp || !resp.enabled) return;
  state.faceCount = resp.unknownCount + resp.autoCount;
  vlens.scheduleRedraw();
}

async function logoutClicked(event: Event) {
  event.preventDefault();
  detachAccountDismissal?.();
  detachAccountDismissal = null;
  await auth.logout();
}
