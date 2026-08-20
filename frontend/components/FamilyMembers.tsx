import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { FamilySelect } from "./FamilySelect";
import "./family-members-styles";

type FamilyMembersState = {
  // Zero means the primary family, matching what the backend falls back to.
  familyId: number;
  members: server.FamilyMemberView[] | null;
  callerIsOwner: boolean;
  error: string;
  busy: boolean;
};

const useFamilyMembers = vlens.declareHook(
  (): FamilyMembersState => ({
    familyId: 0,
    members: null,
    callerIsOwner: false,
    error: "",
    busy: false,
  })
);

async function refresh(state: FamilyMembersState) {
  const [resp, err] = await server.ListFamilyMembers({ familyId: state.familyId });
  if (resp) {
    state.members = resp.members;
    state.callerIsOwner = resp.callerIsOwner;
  } else {
    state.error = err || "Could not load family members";
  }
  vlens.scheduleRedraw();
}

async function onRemoveMember(state: FamilyMembersState, member: server.FamilyMemberView) {
  if (
    !confirm(
      `Remove ${member.name} from this family? They lose access immediately. Anything they added — people, photos, measurements, milestones — stays with the family.`
    )
  ) {
    return;
  }

  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.RemoveFamilyMember({
    familyId: state.familyId,
    userId: member.userId,
  });

  state.busy = false;
  if (resp && resp.success) {
    state.members = resp.members;
    vlens.scheduleRedraw();
    return;
  }
  state.error = resp?.error || err || "Could not remove that member";
  vlens.scheduleRedraw();
}

async function onLeaveFamily(state: FamilyMembersState, familyName: string) {
  if (
    !confirm(
      `Leave ${familyName}? You lose access immediately. Anything you added stays with the family, and you will need a new invite code to come back.`
    )
  ) {
    return;
  }

  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.LeaveFamily({ familyId: state.familyId });

  state.busy = false;
  if (resp && resp.success) {
    // Which families exist and which one is primary have both changed, so the
    // whole page is rebuilt rather than patched.
    window.location.reload();
    return;
  }
  state.error = resp?.error || err || "Could not leave this family";
  vlens.scheduleRedraw();
}

interface FamilyMembersSectionProps {
  initialMembers: server.FamilyMemberView[];
  initialCallerIsOwner: boolean;
  // Name of the family currently selected, for the confirmation prompt.
  familyName: string;
}

// FamilyMembersSection is who is in the household and how somebody stops being
// in it — the two operations that otherwise turn into a support request.
export const FamilyMembersSection = ({
  initialMembers,
  initialCallerIsOwner,
  familyName,
}: FamilyMembersSectionProps): preact.ComponentChild => {
  const state = useFamilyMembers();
  const members = state.members ?? initialMembers;
  const callerIsOwner = state.members ? state.callerIsOwner : initialCallerIsOwner;
  const self = members.find(member => member.isSelf);

  return (
    <div className="settings-section">
      <h2>Family Members</h2>
      <div className="settings-card">
        <p className="section-description">
          Everyone here can see and edit this family's people, photos, measurements and milestones.
          Removing someone — or leaving yourself — takes effect immediately and leaves all of that
          content with the family.
        </p>

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        <FamilySelect
          id="membersFamilyId"
          label="Family"
          value={state.familyId}
          disabled={state.busy}
          onChange={familyId => {
            state.familyId = familyId;
            state.members = null;
            void refresh(state);
          }}
        />

        <ul className="family-members">
          {members.map(member => (
            <li key={member.userId} className="family-member">
              <div className="family-member-identity">
                <strong>{member.name}</strong>
                <span className="family-member-email">{member.email}</span>
              </div>
              <div className="family-member-actions">
                {member.isOwner && <span className="family-member-badge">Owner</span>}
                {member.isSelf && <span className="family-member-badge">You</span>}
                {callerIsOwner && !member.isSelf && (
                  <button
                    type="button"
                    className="btn btn-danger btn-small"
                    disabled={state.busy}
                    onClick={() => onRemoveMember(state, member)}
                  >
                    Remove
                  </button>
                )}
              </div>
            </li>
          ))}
        </ul>

        {self && members.length > 1 && (
          <div className="family-member-leave">
            <button
              type="button"
              className="btn btn-danger"
              disabled={state.busy}
              onClick={() => onLeaveFamily(state, familyName)}
            >
              {state.busy ? "Working..." : "Leave this family"}
            </button>
          </div>
        )}

        {self && members.length === 1 && (
          <p className="section-description">
            You are the only member of this family. To remove it and everything in it, delete your
            account.
          </p>
        )}
      </div>
    </div>
  );
};
