import React, { useState, useEffect, useRef } from 'react';
import styles from './TerminalShowcase.module.css';

export default function TerminalShowcase({ title = 'kgg-admin@workstation:~', steps = [] }) {
  const [visibleLines, setVisibleLines] = useState([]);
  const [currentStepIndex, setCurrentStepIndex] = useState(0);
  const [currentTypedText, setCurrentTypedText] = useState('');
  const containerRef = useRef(null);

  useEffect(() => {
    if (steps.length === 0) return;
    
    let isCancelled = false;
    const runSimulation = async () => {
      // Clear screen
      setVisibleLines([]);
      setCurrentStepIndex(0);
      setCurrentTypedText('');

      for (let i = 0; i < steps.length; i++) {
        if (isCancelled) return;
        const step = steps[i];
        
        if (step.type === 'input') {
          // Type command
          setCurrentStepIndex(i);
          setCurrentTypedText('');
          for (let charIndex = 0; charIndex <= step.text.length; charIndex++) {
            if (isCancelled) return;
            setCurrentTypedText(step.text.slice(0, charIndex));
            await new Promise((resolve) => setTimeout(resolve, 30 + Math.random() * 20));
          }
          // After typing completed, add it to visible lines and clear typed state
          await new Promise((resolve) => setTimeout(resolve, 200));
          setVisibleLines((prev) => [...prev, { type: 'input', text: step.text }]);
          setCurrentTypedText('');
        } else {
          // Output line
          await new Promise((resolve) => setTimeout(resolve, step.delay || 300));
          if (isCancelled) return;
          setVisibleLines((prev) => [...prev, step]);
        }
      }
      
      // Wait before restarting
      await new Promise((resolve) => setTimeout(resolve, 4000));
      if (!isCancelled) {
        runSimulation();
      }
    };

    runSimulation();

    return () => {
      isCancelled = true;
    };
  }, [steps]);

  // Auto-scroll terminal body to bottom
  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [visibleLines, currentTypedText]);

  return (
    <div className={styles.terminalContainer}>
      <div className={styles.terminalHeader}>
        <span className={`${styles.terminalDot} ${styles.dotRed}`}></span>
        <span className={`${styles.terminalDot} ${styles.dotYellow}`}></span>
        <span className={`${styles.terminalDot} ${styles.dotGreen}`}></span>
        <span className={styles.terminalTitle}>{title}</span>
      </div>
      <div className={styles.terminalBody} ref={containerRef}>
        {visibleLines.map((line, idx) => (
          <div key={idx} className={styles.terminalLine}>
            {line.type === 'input' && (
              <>
                <span className={styles.terminalPrompt}>$</span>
                <span className={styles.inputText}>{line.text}</span>
              </>
            )}
            {line.type === 'output' && (
              <span className={styles.outputText} style={line.color ? { color: line.color } : {}}>{line.text}</span>
            )}
            {line.type === 'success' && (
              <span className={styles.successText}>{line.text}</span>
            )}
            {line.type === 'error' && (
              <span className={styles.errorText}>{line.text}</span>
            )}
          </div>
        ))}
        {currentStepIndex < steps.length && steps[currentStepIndex].type === 'input' && (
          <div className={styles.terminalLine}>
            <span className={styles.terminalPrompt}>$</span>
            <span className={styles.inputText}>{currentTypedText}</span>
            <span className={styles.cursor}>█</span>
          </div>
        )}
      </div>
    </div>
  );
}
