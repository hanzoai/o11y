import 'react-day-picker/style.css';
import './index.css';
import { DayPicker } from 'react-day-picker';

import { cn } from '@hanzo/ui/core';

import type { JSX } from 'react';

export type CalendarProps = React.ComponentProps<typeof DayPicker>;

/**
 * The date picker, on react-day-picker's own markup and stylesheet.
 *
 * The class hooks (`periscope-calendar`, `periscope-calendar-day`) are what the
 * app's SCSS already selects on to fit the picker into a popover, so they are
 * applied here rather than repeated at every call site.
 */
export function Calendar({
	className,
	classNames,
	...props
}: CalendarProps): JSX.Element {
	return (
		<DayPicker
			className={cn('periscope-calendar', className)}
			classNames={{ day: 'periscope-calendar-day', ...classNames }}
			{...props}
		/>
	);
}
