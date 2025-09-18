import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";

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

type ProfileData = server.GetPersonResponse | { person: null; growthData: server.GrowthData[] };

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
        <ProfilePage person={data.person} growthData={data.growthData} />
      </main>
      <Footer />
    </div>
  );
}

interface ProfilePageProps {
  person: server.Person;
  growthData: server.GrowthData[];
}

function setActiveTab(state: ProfileState, tab: 'timeline' | 'growth' | 'photos') {
  state.activeTab = tab;
  vlens.scheduleRedraw();
}

const ProfilePage = ({ person, growthData }: ProfilePageProps) => {
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

  const calculateAge = (birthday: string) => {
    if (!birthday) return person.age || 0;
    // Parse birthday as local date to avoid timezone issues
    const dateParts = birthday.split('T')[0].split('-');
    const birthYear = parseInt(dateParts[0]);
    const birthMonth = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const birthDay = parseInt(dateParts[2]);
    const birth = new Date(birthYear, birthMonth, birthDay);

    const today = new Date();
    let age = today.getFullYear() - birth.getFullYear();
    const monthDiff = today.getMonth() - birth.getMonth();
    if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birth.getDate())) {
      age--;
    }
    return age;
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
              {getTypeLabel(person.type)} • Age {calculateAge(person.birthday)}
            </p>
            <p className="profile-birthday">
              Birthday: {formatDate(person.birthday)}
            </p>
          </div>
        </div>

        <div className="profile-actions">
          <button className="btn btn-primary">
            📝 Add Milestone
          </button>
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
        {state.activeTab === 'timeline' && <TimelineTab person={person} />}
        {state.activeTab === 'growth' && <GrowthTab person={person} growthData={growthData} />}
        {state.activeTab === 'photos' && <PhotosTab person={person} />}
      </div>
    </div>
  );
};

const TimelineTab = ({ person }: { person: server.Person }) => {
  return (
    <div className="timeline-tab">
      <h2>Timeline for {person.name}</h2>
      <div className="timeline-content">
        <div className="empty-state">
          <p>No timeline entries yet.</p>
          <button className="btn btn-primary">Add First Milestone</button>
        </div>
      </div>
    </div>
  );
};

const GrowthTab = ({ person, growthData }: { person: server.Person; growthData: server.GrowthData[] }) => {
  const getMeasurementTypeLabel = (type: server.MeasurementType) => {
    return type === server.Height ? 'Height' : 'Weight';
  };


  // Sort growth data by measurement date (newest first)
  const sortedGrowthData = (growthData || []).slice().sort((a, b) =>
    new Date(b.measurementDate).getTime() - new Date(a.measurementDate).getTime()
  );

  return (
    <div className="growth-tab">
      <h2>Growth Tracking for {person.name}</h2>
      <div className="growth-content">
        <div className="growth-chart-placeholder">
          <h3>Growth Chart</h3>
          <div className="chart-placeholder">
            <p>📈 Growth chart will be displayed here</p>
          </div>
        </div>

        <div className="growth-table">
          <h3>Growth Records</h3>
          {sortedGrowthData.length === 0 ? (
            <div className="empty-state">
              <p>No growth records yet.</p>
              <a href={`/add-growth/${person.id}`} className="btn btn-primary">Add First Measurement</a>
            </div>
          ) : (
            <div className="growth-records">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Type</th>
                    <th>Value</th>
                    <th>Date</th>
                    <th>Added</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedGrowthData.map(record => (
                    <tr key={record.id}>
                      <td>{getMeasurementTypeLabel(record.measurementType)}</td>
                      <td>{record.value} {record.unit}</td>
                      <td>{formatDate(record.measurementDate)}</td>
                      <td>{formatDate(record.createdAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="table-actions">
                <a href={`/add-growth/${person.id}`} className="btn btn-primary">Add New Measurement</a>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

const PhotosTab = ({ person }: { person: server.Person }) => {
  return (
    <div className="photos-tab">
      <h2>Photos of {person.name}</h2>
      <div className="photos-content">
        <div className="photos-gallery">
          <div className="empty-state">
            <p>No photos yet.</p>
            <button className="btn btn-primary">Add First Photo</button>
          </div>
        </div>
      </div>
    </div>
  );
};
