import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { UnifiedTimeline } from "./tabs/unified-timeline";
import { ProfileImage } from "../../components/ResponsiveImage";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import "./profile-styles";

type ProfileState = {
  visibleTypes: {
    milestones: boolean;
    measurements: boolean;
    photos: boolean;
  };
  selectedAgeFilter: string; // "all" or year number as string like "0", "1", "2"
  sortOrder: "newest" | "oldest";
};

const useProfileState = vlens.declareHook(
  (): ProfileState => ({
    visibleTypes: {
      milestones: true,
      measurements: true,
      photos: true,
    },
    selectedAgeFilter: "all",
    sortOrder: "newest",
  })
);

export async function fetch(route: string, prefix: string) {
  const personId = parseInt(route.split("/")[2]);
  return server.GetPerson({ id: personId });
}

type ProfileData =
  | server.GetPersonResponse
  | { person: null; growthData: server.GrowthData[]; milestones: server.Milestone[] };

const formatDate = (dateString: string) => {
  if (!dateString) return "";
  if (dateString.includes("T") && dateString.endsWith("Z")) {
    const dateParts = dateString.split("T")[0].split("-");
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

export function view(route: string, prefix: string, data: ProfileData): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute("/login");
    return;
  }

  if (!data.person) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="profile-container">
          <div className="error-page">
            <h1>Error</h1>
            <p>Failed to load person data</p>
            <a href="/dashboard" className="btn btn-primary">
              Back to Dashboard
            </a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="profile-container">
        <ProfilePage
          person={data.person}
          growthData={data.growthData}
          milestones={data.milestones}
          photos={data.photos}
        />
      </main>
      <Footer />
    </div>
  );
}

interface ProfilePageProps {
  person: server.Person;
  growthData: server.GrowthData[];
  milestones: server.Milestone[];
  photos: server.Image[];
}

function toggleType(state: ProfileState, type: "milestones" | "measurements" | "photos") {
  state.visibleTypes[type] = !state.visibleTypes[type];
  vlens.scheduleRedraw();
}

function setAgeFilter(state: ProfileState, filter: string) {
  state.selectedAgeFilter = filter;
  vlens.scheduleRedraw();
}

function setSortOrder(state: ProfileState, order: "newest" | "oldest") {
  state.sortOrder = order;
  vlens.scheduleRedraw();
}

const ProfilePage = ({ person, growthData, milestones, photos }: ProfilePageProps) => {
  const state = useProfileState();
  const photoStatus = usePhotoStatus();

  const getGenderIcon = (gender: number) => {
    switch (gender) {
      case 0:
        return "👨";
      case 1:
        return "👩";
      default:
        return "👤";
    }
  };

  const getTypeLabel = (type: number) => {
    return type === 0 ? "Parent" : "Child";
  };

  return (
    <div className="profile-page">
      {/* Profile Header */}
      <div className="profile-header">
        <div className="profile-header-main">
          <div className="profile-avatar">
            {person.profilePhotoId ? (
              <ProfileImage
                photoId={person.profilePhotoId}
                alt={`${person.name}'s profile photo`}
                className="profile-photo"
                loading="eager"
                fetchpriority="high"
                status={photoStatus.getStatus(person.profilePhotoId)}
              />
            ) : (
              <span className="profile-icon">{getGenderIcon(person.gender)}</span>
            )}
          </div>
          <div className="profile-info">
            <h1>{person.name}</h1>
            <p className="profile-details">
              {getTypeLabel(person.type)} • Age {person.age}
            </p>
            <p className="profile-birthday">Birthday: {formatDate(person.birthday)}</p>
          </div>
        </div>

        <div className="profile-actions">
          <a href={`/add-milestone/${person.id}`} className="btn btn-primary">
            📝 Add Milestone
          </a>
          <a href={`/add-growth/${person.id}`} className="btn btn-primary">
            📏 Measure Now
          </a>
          <a href={`/add-photo/${person.id}`} className="btn btn-primary">
            📸 Add Photo
          </a>
        </div>
      </div>

      {/* Type Filter Controls */}
      <div className="profile-filters">
        <div className="filter-section">
          <label className="filter-label">Show:</label>
          <div className="type-filters">
            <button
              className={`filter-toggle ${state.visibleTypes.milestones ? "active" : ""}`}
              onClick={() => toggleType(state, "milestones")}
            >
              {state.visibleTypes.milestones ? "✓" : ""} Milestones
            </button>
            <button
              className={`filter-toggle ${state.visibleTypes.measurements ? "active" : ""}`}
              onClick={() => toggleType(state, "measurements")}
            >
              {state.visibleTypes.measurements ? "✓" : ""} Measurements
            </button>
            <button
              className={`filter-toggle ${state.visibleTypes.photos ? "active" : ""}`}
              onClick={() => toggleType(state, "photos")}
            >
              {state.visibleTypes.photos ? "✓" : ""} Photos
            </button>
          </div>
        </div>
        <div className="filter-section sort-section">
          <label className="filter-label">Sort:</label>
          <div className="sort-controls">
            <button
              className={`filter-toggle ${state.sortOrder === "newest" ? "active" : ""}`}
              onClick={() => setSortOrder(state, "newest")}
            >
              Newest First
            </button>
            <button
              className={`filter-toggle ${state.sortOrder === "oldest" ? "active" : ""}`}
              onClick={() => setSortOrder(state, "oldest")}
            >
              Oldest First
            </button>
          </div>
        </div>
      </div>

      {/* Unified Timeline Content */}
      <div className="profile-content">
        <UnifiedTimeline
          person={person}
          milestones={milestones}
          growthData={growthData}
          photos={photos}
          visibleTypes={state.visibleTypes}
          selectedAgeFilter={state.selectedAgeFilter}
          sortOrder={state.sortOrder}
          onAgeFilterChange={filter => setAgeFilter(state, filter)}
        />
      </div>
    </div>
  );
};
