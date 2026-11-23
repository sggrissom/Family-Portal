import type * as server from "../server";

export const maxPhotos = 6;

/** The slug is exactly the one segment after /f/. */
export function familyPageSlug(route: string): string {
  const path = route.split(/[?#]/)[0];
  const slug = path.startsWith("/f/") ? path.slice("/f/".length).toLowerCase() : "";
  return /^[a-z0-9]+(-[a-z0-9]+)*$/.test(slug) ? slug : "";
}

export function toggleMember(
  form: server.FamilyPageSettings,
  people: server.FamilyPagePerson[],
  person: server.FamilyPagePerson
) {
  const included = form.members.some(member => member.personId === person.id);
  const byId = new Map(form.members.map(member => [member.personId, member]));
  if (included) {
    byId.delete(person.id);
  } else {
    byId.set(person.id, {
      personId: person.id,
      name: person.name.split(/\s+/)[0] || person.name,
      line: "",
      showAge: false,
      showPhoto: false,
    });
  }
  form.members = people.flatMap(p => byId.get(p.id) ?? []);
}

export function togglePhoto(form: server.FamilyPageSettings, photoId: number) {
  if (form.photoIds.includes(photoId)) {
    form.photoIds = form.photoIds.filter(id => id !== photoId);
  } else if (form.photoIds.length < maxPhotos) {
    form.photoIds = [...form.photoIds, photoId];
  }
}

/** What visitors will see, drawn from the form and the signed-in photo URLs. */
export function previewPage(
  form: server.FamilyPageSettings,
  people: server.FamilyPagePerson[]
): server.PublicFamilyPage {
  const byId = new Map(people.map(person => [person.id, person]));
  return {
    title: form.title.trim(),
    intro: form.intro.trim(),
    members: form.members.flatMap(member => {
      const person = byId.get(member.personId);
      if (!person) return [];
      const photo = member.showPhoto && person.profilePhotoId > 0;
      return [
        {
          name: member.name.trim() || person.name.split(/\s+/)[0] || person.name,
          line: member.line.trim(),
          ageYears: member.showAge ? person.ageYears : null,
          photoUrl: photo ? `/api/photo/${person.profilePhotoId}/medium` : "",
          cropX: photo ? person.profileCropX : 0,
          cropY: photo ? person.profileCropY : 0,
          cropScale: photo ? person.profileCropScale : 0,
        },
      ];
    }),
    photoUrls: form.photoIds.map(id => `/api/photo/${id}/large`),
  };
}
