import React from 'react';
import Joyride from 'react-joyride';
import type { CallBackProps } from 'react-joyride';
import { TOUR_STEPS } from '../tour/steps';
import { TOUR_STYLES } from '../tour/styles';
import '../tour/tour.css';

interface TourProps {
  run: boolean;
  onComplete: () => void;
}

export const Tour: React.FC<TourProps> = ({ run, onComplete }) => {
  const handleJoyrideCallback = (data: CallBackProps) => {
    const { status } = data;
    const finishedStatuses = ['finished', 'skipped'];

    if (finishedStatuses.includes(status)) {
      onComplete();
    }

    if (status === 'error') {
      onComplete();
    }
  };

  return (
    <Joyride
      steps={TOUR_STEPS}
      styles={TOUR_STYLES}
      run={run}
      callback={handleJoyrideCallback}
      continuous={true} // Sequential flow through all steps
      scrollToFirstStep={true} // Accessibility: scroll to first step
      showProgress={true} // Show "Step X of 9" indicator
      showSkipButton={true} // User control: allow skipping the tour
      disableScrolling={false} // Allow natural scrolling to steps
      spotlightClicks={false} // Prevent clicking through spotlight overlay
      disableOverlayClose={false} // Allow closing by clicking overlay
      hideBackButton={false} // Show back button for navigation
      scrollDuration={300} // Smooth scroll animation (300ms - subtle, not distracting)
      scrollOffset={20} // Offset from top when scrolling to elements (prevents header overlap)
      floaterProps={{
        disableAnimation: false, // Enable smooth tooltip positioning animations
        hideArrow: false, // Show arrow for better visual connection to target
      }}
      locale={{
        back: 'Back',
        close: 'Close',
        last: 'Finish', // Changed from default "Last" to "Finish" for professional tone
        next: 'Next',
        open: 'Open',
        skip: 'Skip',
      }}
    />
  );
};
