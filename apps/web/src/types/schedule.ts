/**
 * Unified Schedule Data Contract Types
 * Corresponds to schema/schedule.schema.json (Draft-07)
 */

export interface TargetGroup {
  canonical_id?: string;
  name: string;
  short_name?: string;
  official_x?: string;
  official_tiktok?: string;
}

export interface FeaturedMember {
  canonical_id?: string;
  name?: string;
  member_color?: string;
  color_code?: string;
  birthday_title?: string;
  crowdfunding_url?: string;
  banner_image?: string;
}

export interface Promotion {
  id: string;
  title: string;
  subtitle?: string;
  image_url: string;
  cta_url: string;
  deadline?: string;
}

export interface Venue {
  canonical_id?: string;
  name: string;
  map_url?: string;
  map_query?: string;
  official_url?: string;
}

export interface LivePerformance {
  start_time: string;
  end_time?: string;
  stage?: string;
}

export interface BenefitEvent {
  type?: string;
  start_time: string;
  end_time?: string;
  booth?: string;
  note?: string;
}

export interface PhotoRule {
  available?: boolean;
  rule?: string;
  raw_text?: string;
}

export interface TicketPrice {
  tier?: string;
  price?: string;
}

export interface TicketInfo {
  platform?: string;
  url?: string | null;
  prices?: TicketPrice[];
}

export interface SessionImage {
  type: 'flyer' | 'timetable';
  label?: string;
  url: string;
}

export interface SourceRef {
  type: string;
  source_id?: string;
  url: string;
}

export interface Session {
  id: string;
  title: string;
  date: string; // YYYY-MM-DD
  day_of_week?: string;
  session_name?: string;
  venue: Venue;
  live: LivePerformance;
  benefit: BenefitEvent;
  photo?: PhotoRule | null;
  special_notes?: string | null;
  admission_bonus?: string | null;
  ticket?: TicketInfo;
  image_url?: string;
  source_tweet_url?: string;
  images?: SessionImage[];
  source_refs?: SourceRef[];
}

export interface ScheduleData {
  schema_version: number;
  generated_at: string;
  timezone: string;
  current_next_id?: string;
  target_group: TargetGroup;
  featured_member?: FeaturedMember;
  promotions?: Promotion[];
  sessions: Session[];
}
