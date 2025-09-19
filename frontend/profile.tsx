import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";
import { TimelineTab } from "./profile-tabs/timeline";
import { GrowthTab } from "./profile-tabs/growth";
import { PhotosTab } from "./profile-tabs/photos";

type ProfileState = {
  activeTab: 'timeline' | 'growth' | 'photos';
}

const useProfileState = vlens.declareHook((): ProfileState => ({
  activeTab: 'timeline'
}));

export async function fetch(route: string, prefix: string) {
  const personId = parseInt(route.split('/')[2]);
  return server.GetPerson({ id: personId });
}

type ProfileData = server.GetPersonResponse | { person: null; growthData: server.GrowthData[]; milestones: server.Milestone[] };

const formatDate = (dateString: string) => {
  if (!dateString) return '';
  if (dateString.includes('T') && dateString.endsWith('Z')) {
    const dateParts = dateString.split('T')[0].split('-');
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

export function view(
  route: string,
  prefix: string,
  data: ProfileData,
): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute('/login');
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
            <a href="/dashboard" className="btn btn-primary">Back to Dashboard</a>
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
        <ProfilePage person={data.person} growthData={data.growthData} milestones={data.milestones} />
      </main>
      <Footer />
    </div>
  );
}

interface ProfilePageProps {
  person: server.Person;
  growthData: server.GrowthData[];
  milestones: server.Milestone[];
}

function setActiveTab(state: ProfileState, tab: 'timeline' | 'growth' | 'photos') {
  state.activeTab = tab;
  vlens.scheduleRedraw();
}

const ProfilePage = ({ person, growthData, milestones }: ProfilePageProps) => {
  const state = useProfileState();

  const getGenderIcon = (gender: number) => {
    switch (gender) {
      case 0: return "👨";
      case 1: return "👩";
      default: return "👤";
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
            {getGenderIcon(person.gender)}
          </div>
          <div className="profile-info">
            <h1>{person.name}</h1>
            <p className="profile-details">
              {getTypeLabel(person.type)} • Age {person.age}
            </p>
            <p className="profile-birthday">
              Birthday: {formatDate(person.birthday)}
            </p>
          </div>
        </div>

        <div className="profile-actions">
          <a href={`/add-milestone/${person.id}`} className="btn btn-primary">
            📝 Add Milestone
          </a>
          <a href={`/add-growth/${person.id}`} className="btn btn-primary">
            📏 Add Growth
          </a>
          <button className="btn btn-primary">
            📸 Add Photo
          </button>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="profile-tabs">
        <button
          className={`tab ${state.activeTab === 'timeline' ? 'active' : ''}`}
          onClick={() => setActiveTab(state, 'timeline')}
        >
          📰 Timeline
        </button>
        <button
          className={`tab ${state.activeTab === 'growth' ? 'active' : ''}`}
          onClick={() => setActiveTab(state, 'growth')}
        >
          📊 Growth
        </button>
        <button
          className={`tab ${state.activeTab === 'photos' ? 'active' : ''}`}
          onClick={() => setActiveTab(state, 'photos')}
        >
          🖼️ Photos
        </button>
      </div>

      {/* Tab Content */}
      <div className="profile-content">
        {state.activeTab === 'timeline' && <TimelineTab person={person} milestones={milestones} />}
        {state.activeTab === 'growth' && <GrowthTab person={person} growthData={growthData} />}
        {state.activeTab === 'photos' && <PhotosTab person={person} />}
      </div>
    </div>
  );
};
