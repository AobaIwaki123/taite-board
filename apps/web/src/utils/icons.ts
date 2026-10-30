/**
 * Optimized Lucide icon loader
 * Imports only specifically used icons to minimize bundle footprint.
 */

import {
  createIcons,
  Aperture,
  Calendar,
  CalendarDays,
  Camera,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Gift,
  Image,
  List,
  MapPin,
  Maximize2,
  Mic,
  Navigation,
  Sparkles,
  Ticket,
} from 'lucide';

const usedIcons = {
  Aperture,
  Calendar,
  CalendarDays,
  Camera,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Gift,
  Image,
  List,
  MapPin,
  Maximize2,
  Mic,
  Navigation,
  Sparkles,
  Ticket,
};

export function refreshLucideIcons(): void {
  try {
    createIcons({
      icons: usedIcons,
    });
  } catch (e) {
    console.warn('Failed to refresh Lucide icons', e);
  }
}
