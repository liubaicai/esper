import com.espertech.esper.common.internal.support.SupportBean;

/**
 * Default-package stand-in for the regression-lib ContextHashSegmented class:
 * the single-row-func case EPL references ContextHashSegmented.mySecondFunc by
 * simple class name, so the oracle provides the same public static functions.
 */
public final class ContextHashSegmented {

    private ContextHashSegmented() {
    }

    /** Mirrors ContextHashSegmented.myHashFunc: returns the bean's intPrimitive. */
    public static int myHashFunc(SupportBean event) {
        return event.getIntPrimitive();
    }

    /** Mirrors ContextHashSegmented.mySecondFunc: returns the text argument. */
    public static String mySecondFunc(SupportBean event, String text) {
        return text;
    }
}
