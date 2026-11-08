import { Images, MessageCircle, Mic, Palette, Shuffle, Star, Trophy } from "lucide-preact";
import type { LucideIcon } from "lucide-preact";
import * as preact from "preact";
import * as server from "../server";
import { Icon } from "./Icon";
import { ResponsiveImage, ThumbnailImage } from "./ResponsiveImage";
import { copy } from "../lib/copy";
import { shortQuestion } from "../lib/interviews";
import { memoryHref, memoryNames, memoryWhen } from "../lib/memories";
import "./memory-card-styles";

const KIND_ICONS: Record<string, LucideIcon> = {
  moment: Star,
  quote: MessageCircle,
  artwork: Palette,
  interview: Mic,
  event: Trophy,
  photo: Images,
};

const kindIcon = (kind: string) => KIND_ICONS[kind] ?? Star;

const MemoryText = ({ memory }: { memory: server.Memory }) =>
  memory.kind === "quote" ? <q>{memory.text}</q> : <>{memory.text}</>;

function lead(memory: server.Memory): string {
  if (memory.kind === "interview") return shortQuestion(memory.context);
  return "";
}

export const FeaturedMemory = ({
  memory,
  people,
  onAnother,
}: {
  memory: server.Memory;
  people: Map<number, server.Person>;
  onAnother?: () => void;
}) => {
  const photoId = memory.photoIds?.[0] ?? 0;
  const names = memoryNames(memory, people);
  const question = lead(memory);
  return (
    <article className={photoId ? "memory-featured" : "memory-featured no-photo"}>
      {photoId > 0 && (
        <a href={memoryHref(memory)} className="memory-featured-photo" tabIndex={-1}>
          <ResponsiveImage
            photoId={photoId}
            alt=""
            sizes="(max-width: 760px) 100vw, 360px"
            loading="eager"
          />
        </a>
      )}
      <div className="memory-featured-body">
        <span className="memory-eyebrow">
          <Icon symbol={kindIcon(memory.kind)} /> {copy.memory.kinds[memory.kind] ?? ""}
          {names && ` · ${names}`}
        </span>
        {question && <span className="memory-question">{question}</span>}
        {memory.text && (
          <a href={memoryHref(memory)} className="memory-featured-text">
            <MemoryText memory={memory} />
          </a>
        )}
        {memory.kind !== "interview" && memory.context && (
          <p className="memory-context">{memory.context}</p>
        )}
        <span className="memory-when">{memoryWhen(memory, people)}</span>
        <div className="memory-actions">
          <a href={memoryHref(memory)} className="btn btn-secondary">
            {copy.memory.open}
          </a>
          {onAnother && (
            <button type="button" className="btn btn-secondary" onClick={onAnother}>
              <Icon symbol={Shuffle} /> {copy.memory.another}
            </button>
          )}
        </div>
      </div>
    </article>
  );
};

export const MemoryTile = ({
  memory,
  people,
}: {
  memory: server.Memory;
  people: Map<number, server.Person>;
}) => {
  const photoId = memory.photoIds?.[0] ?? 0;
  const names = memoryNames(memory, people);
  return (
    <a href={memoryHref(memory)} className="memory-tile">
      {photoId > 0 ? (
        <ThumbnailImage photoId={photoId} alt="" className="memory-tile-photo" />
      ) : (
        <span className="memory-tile-icon" aria-hidden="true">
          <Icon symbol={kindIcon(memory.kind)} />
        </span>
      )}
      <span className="memory-tile-body">
        <span className="memory-tile-text">
          {memory.text ? <MemoryText memory={memory} /> : (copy.memory.kinds[memory.kind] ?? "")}
        </span>
        <span className="memory-tile-meta">
          {names && `${names} · `}
          {memoryWhen(memory, people)}
        </span>
      </span>
    </a>
  );
};

export const MemoryTiles = ({
  memories,
  people,
}: {
  memories: server.Memory[];
  people: Map<number, server.Person>;
}) => (
  <div className="memory-tiles">
    {memories.map(memory => (
      <MemoryTile key={memory.key} memory={memory} people={people} />
    ))}
  </div>
);
