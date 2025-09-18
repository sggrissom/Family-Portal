import * as preact from "preact";
import { useState } from "preact/hooks";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";

export async function fetch(route: string, prefix: string) {
  const personId = parseInt(route.split('/')[2]);
  return server.GetPerson({ id: personId });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetPersonResponse,
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
        <ProfilePage person={data.person} />
      </main>
      <Footer />
    </div>
  );
}

interface ProfilePageProps {
  person: server.Person;
}

const ProfilePage = ({ person }: ProfilePageProps) => {
  const [activeTab, setActiveTab] = useState<'timeline' | 'growth' | 'photos'>('timeline');

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
    const birth = new Date(birthday);
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
          </div>
        </div>

        <div className="profile-actions">
          <button className="btn btn-primary">
            📝 Add Milestone
          </button>
          <button className="btn btn-primary">
            📏 Add Growth
          </button>
          <button className="btn btn-primary">
            📸 Add Photo
          </button>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="profile-tabs">
        <button
          className={`tab ${activeTab === 'timeline' ? 'active' : ''}`}
          onClick={() => setActiveTab('timeline')}
        >
          📰 Timeline
        </button>
        <button
          className={`tab ${activeTab === 'growth' ? 'active' : ''}`}
          onClick={() => setActiveTab('growth')}
        >
          📊 Growth
        </button>
        <button
          className={`tab ${activeTab === 'photos' ? 'active' : ''}`}
          onClick={() => setActiveTab('photos')}
        >
          🖼️ Photos
        </button>
      </div>

      {/* Tab Content */}
      <div className="profile-content">
        {activeTab === 'timeline' && <TimelineTab person={person} />}
        {activeTab === 'growth' && <GrowthTab person={person} />}
        {activeTab === 'photos' && <PhotosTab person={person} />}
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

const GrowthTab = ({ person }: { person: server.Person }) => {
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
          <div className="empty-state">
            <p>No growth records yet.</p>
            <button className="btn btn-primary">Add First Measurement</button>
          </div>
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
