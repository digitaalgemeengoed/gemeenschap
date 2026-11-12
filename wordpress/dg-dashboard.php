<?php
/**
 * Plugin Name: Digitaal GemeenGoed — dashboard voor leden
 * Description: Wat een ingelogd lid op de startpagina ziet: profiel en navigatie, projectupdates uit Redmine, recente discussies uit de fora, samenwerkingen, actieve leden en evenementen uit CiviCRM. Shortcode [dg_dashboard].
 *
 * Op de startpagina (inhoud/paginas/home.md) staat [dg_dashboard] onder de
 * opening. Voor wie niet is ingelogd, geeft de shortcode niets. Voor wie wel
 * is ingelogd, toont het thema alles wat erna komt niet: dat is voor gasten.
 *
 * @package DigitaalGemeenGoed
 */

defined( 'ABSPATH' ) || exit;

/** Hoe lang de gegevens uit Redmine bewaard blijven (seconden). */
const DG_DASHBOARD_CACHE = 600;

add_shortcode( 'dg_dashboard', 'dg_dashboard' );

/**
 * [dg_fase project="xena"]: de fase van een project uit Redmine (veld Fase),
 * of "nog niet bepaald".
 */
add_shortcode(
	'dg_fase',
	static function ( $attr ) {
		$id    = sanitize_title( (string) ( $attr['project'] ?? '' ) );
		$fasen = get_transient( 'dg_db_fasen' );
		if ( ! is_array( $fasen ) ) {
			$fasen = array();
			foreach ( ( dg_db_redmine( '/projects.json?limit=100' ) ?? array() )['projects'] ?? array() as $p ) {
				foreach ( $p['custom_fields'] ?? array() as $veld ) {
					if ( 'Fase' === ( $veld['name'] ?? '' ) && ! empty( $veld['value'] ) ) {
						$fasen[ (string) $p['identifier'] ] = (string) $veld['value'];
					}
				}
			}
			set_transient( 'dg_db_fasen', $fasen, DG_DASHBOARD_CACHE );
		}
		$fase = $fasen[ $id ] ?? '';
		return $fase ? '<span class="dg-db-fase dg-db-fase-' . esc_attr( sanitize_title( $fase ) ) . '">' . esc_html( $fase ) . '</span>' : 'nog niet bepaald';
	}
);

/** Het adres van Documenten (documenten.<domein>), met een pad erachter. */
function dg_documenten_url( string $pad = '' ): string {
	return 'https://documenten.' . wp_parse_url( home_url(), PHP_URL_HOST ) . '/' . ltrim( $pad, '/' );
}

/**
 * [dg_documenten project="xena"]: een link naar de documenten van het project
 * (de ruimte in Documenten heet zoals het project in Redmine). Alleen voor
 * wie is ingelogd; anderen zien "voor leden".
 */
add_shortcode(
	'dg_documenten',
	static function ( $attr ) {
		$id = sanitize_title( (string) ( $attr['project'] ?? '' ) );
		if ( '' === $id ) {
			return '';
		}
		if ( ! is_user_logged_in() ) {
			return '<span class="dg-voor-leden">voor leden</span>';
		}
		return '<a href="' . esc_url( dg_documenten_url( 'r/' . $id ) ) . '">Documenten van dit project</a>';
	}
);

/** Het hele dashboard. */
function dg_dashboard(): string {
	if ( ! is_user_logged_in() ) {
		return '';
	}
	$gebruiker = wp_get_current_user();
	ob_start();
	?>
	<div class="dg-dashboard">
		<aside class="dg-db-links">
			<?php echo dg_db_profiel( $gebruiker ); // phpcs:ignore WordPress.Security.EscapeOutput -- opgebouwd met esc_*. ?>
			<?php echo dg_db_navigatie( $gebruiker ); // phpcs:ignore WordPress.Security.EscapeOutput ?>
		</aside>
		<div class="dg-db-midden">
			<section class="dg-db-welkom">
				<span class="dg-icoon dg-icoon-mensen" aria-hidden="true"></span>
				<div>
					<h2>Welkom in de DGG-community!</h2>
					<p>Dé plek voor gemeentelijke professionals, developers, beheerders en beleidsmakers die samen werken aan open, betrouwbare en herbruikbare overheidssoftware.</p>
				</div>
			</section>
			<div class="dg-db-raster">
				<?php echo dg_db_projectupdates(); // phpcs:ignore WordPress.Security.EscapeOutput ?>
				<?php echo dg_db_discussies(); // phpcs:ignore WordPress.Security.EscapeOutput ?>
				<section class="dg-db-blok dg-db-oproep dg-db-kennis">
					<h3><span class="dg-icoon dg-icoon-boek" aria-hidden="true"></span>Kennis- &amp; ervaringsbank</h3>
					<p>Handleidingen, how-to's, referentiearchitecturen, lessen uit de praktijk en herbruikbare componenten.</p>
					<a class="dg-db-knop dg-db-knop-rand" href="<?php echo esc_url( dg_db_kennisbank() ); ?>">Naar de kennisbank ›</a>
				</section>
				<section class="dg-db-blok dg-db-oproep dg-db-doemee">
					<h3><span class="dg-icoon dg-icoon-hand" aria-hidden="true"></span>Doe mee!</h3>
					<p>Deel je kennis, stel vragen en werk mee aan open source oplossingen voor alle gemeenten.</p>
					<a class="dg-db-knop" href="<?php echo esc_url( dg_db_forum_url() ); ?>">Start een discussie ›</a>
				</section>
			</div>
		</div>
		<aside class="dg-db-rechts">
			<?php echo dg_db_samenwerkingen(); // phpcs:ignore WordPress.Security.EscapeOutput ?>
			<?php echo dg_db_leden(); // phpcs:ignore WordPress.Security.EscapeOutput ?>
			<?php echo dg_db_evenementen(); // phpcs:ignore WordPress.Security.EscapeOutput ?>
		</aside>
	</div>
	<?php
	return (string) ob_get_clean();
}

/** Een blok met kop, icoon en een link rechtsboven. */
function dg_db_blok( string $klasse, string $icoon, string $titel, string $link_tekst, string $link, string $inhoud ): string {
	$link_html = $link ? '<a class="dg-db-alles" href="' . esc_url( $link ) . '">' . esc_html( $link_tekst ) . ' ›</a>' : '';
	return '<section class="dg-db-blok ' . esc_attr( $klasse ) . '"><header><h3><span class="dg-icoon dg-icoon-' . esc_attr( $icoon ) . '" aria-hidden="true"></span>' . $titel . '</h3>' . $link_html . '</header>' . $inhoud . '</section>';
}

/** "10 min", "2 uur", "gisteren", of de datum. */
function dg_db_wanneer( int $tijd ): string {
	$verschil = time() - $tijd;
	if ( $verschil < HOUR_IN_SECONDS ) {
		return max( 1, (int) round( $verschil / MINUTE_IN_SECONDS ) ) . ' min';
	}
	if ( $verschil < DAY_IN_SECONDS ) {
		return (int) round( $verschil / HOUR_IN_SECONDS ) . ' uur';
	}
	if ( $verschil < 2 * DAY_IN_SECONDS ) {
		return 'gisteren';
	}
	return wp_date( 'j M', $tijd );
}

// ---------------------------------------------------------------------------
// Links: profiel en navigatie
// ---------------------------------------------------------------------------

function dg_db_profiel( WP_User $gebruiker ): string {
	$foto   = function_exists( 'bp_core_fetch_avatar' )
		? bp_core_fetch_avatar( array( 'item_id' => $gebruiker->ID, 'type' => 'full', 'width' => 64, 'height' => 64, 'html' => true, 'alt' => '' ) )
		: get_avatar( $gebruiker->ID, 64 );
	$profiel = function_exists( 'bp_members_get_user_url' ) ? bp_members_get_user_url( $gebruiker->ID ) : admin_url( 'profile.php' );
	$org     = (string) get_user_meta( $gebruiker->ID, 'dg_organisatie', true );
	return '<div class="dg-db-profiel">' . $foto . '<div><strong>' . esc_html( $gebruiker->display_name ) . '</strong>'
		. ( $org ? '<span>' . esc_html( $org ) . '</span>' : '' )
		. '<a href="' . esc_url( $profiel ) . '">Mijn profiel ›</a></div></div>';
}

function dg_db_forum_url(): string {
	return function_exists( 'bbp_get_forums_url' ) ? bbp_get_forums_url() : home_url( '/forum/' );
}

function dg_db_navigatie( WP_User $gebruiker ): string {
	$berichten = home_url( '/' );
	$ongelezen = 0;
	if ( function_exists( 'bp_is_active' ) && bp_is_active( 'messages' ) ) {
		$berichten = function_exists( 'bp_members_get_user_url' ) ? bp_members_get_user_url( $gebruiker->ID, bp_members_get_path_chunks( array( bp_get_messages_slug() ) ) ) : $berichten;
		$ongelezen = (int) messages_get_unread_count( $gebruiker->ID );
	}
	$keycloak = function_exists( 'dg_keycloak' ) && dg_keycloak() ? dg_keycloak() . '/realms/gemeenschap/account/' : admin_url( 'profile.php' );
	$punten   = array(
		array( 'huis', 'Dashboard', home_url( '/' ), 0 ),
		array( 'mensen', 'Community', function_exists( 'bp_get_members_directory_permalink' ) ? bp_get_members_directory_permalink() : home_url( '/leden/' ), 0 ),
		array( 'forum', 'Discussies & fora', dg_db_forum_url(), 0 ),
		array( 'boek', 'Kennis- & ervaringsbank', dg_db_kennisbank(), 0 ),
		array( 'document', 'Documenten', dg_documenten_url(), 0 ),
		array( 'taken', 'Taken', function_exists( 'dg_pv_taken_url' ) ? dg_pv_taken_url() : home_url( '/' ), 0 ),
		array( 'lagen', 'Projecten', home_url( '/projecten/' ), 0 ),
		array( 'hand', 'Samenwerkingen', home_url( '/samenwerkingen/' ), 0 ),
		array( 'kalender', 'Evenementen', dg_db_evenementen_url(), 0 ),
		array( 'mail', 'Berichten', $berichten, $ongelezen ),
		array( 'chat', 'Chat', function_exists( 'dg_chat_url' ) ? dg_chat_url() : home_url( '/' ), 0 ),
		array( 'instellingen', 'Instellingen', $keycloak, 0 ),
	);
	$html = '<nav class="dg-db-nav" aria-label="Dashboard"><ul>';
	foreach ( $punten as $nr => [ $icoon, $label, $url, $teller ] ) {
		$html .= '<li' . ( 0 === $nr ? ' class="is-actief"' : '' ) . '><a href="' . esc_url( $url ) . '"><span class="dg-icoon dg-icoon-' . $icoon . '" aria-hidden="true"></span>' . esc_html( $label )
			. ( $teller ? '<span class="dg-db-teller" aria-label="' . esc_attr( $teller . ' ongelezen' ) . '">' . (int) $teller . '</span>' : '' ) . '</a></li>';
	}
	return $html . '</ul></nav>';
}

// ---------------------------------------------------------------------------
// Midden: projectupdates (Redmine, nieuws van openbare projecten) en discussies
// ---------------------------------------------------------------------------

/** Een GET op de API van Redmine, via het interne adres; null als het niet lukt. */
function dg_db_redmine( string $pad ): ?array {
	$basis = rtrim( (string) ( getenv( 'DG_REDMINE_INTERN' ) ?: '' ), '/' );
	if ( ! $basis ) {
		return null;
	}
	$antwoord = wp_remote_get( $basis . $pad, array( 'timeout' => 5, 'headers' => array( 'Accept' => 'application/json' ) ) );
	if ( is_wp_error( $antwoord ) || 200 !== wp_remote_retrieve_response_code( $antwoord ) ) {
		return null;
	}
	$data = json_decode( wp_remote_retrieve_body( $antwoord ), true );
	return is_array( $data ) ? $data : null;
}

// Het interne adres van Redmine mag WordPress opvragen.
add_filter(
	'http_request_host_is_external',
	static function ( $extern, $host ) {
		$intern = wp_parse_url( (string) getenv( 'DG_REDMINE_INTERN' ), PHP_URL_HOST );
		return $intern && $host === $intern ? true : $extern;
	},
	10,
	2
);

/** De laatste drie nieuwsberichten van de openbare projecten, met de fase van het project. */
function dg_db_projectupdates_gegevens(): array {
	$cache = get_transient( 'dg_db_projectupdates' );
	if ( is_array( $cache ) ) {
		return $cache;
	}
	$nieuws    = dg_db_redmine( '/news.json?limit=3' );
	$projecten = dg_db_redmine( '/projects.json?limit=100' );
	if ( null === $nieuws ) {
		set_transient( 'dg_db_projectupdates', array(), 60 );
		return array();
	}
	$fase = array();
	foreach ( $projecten['projects'] ?? array() as $p ) {
		foreach ( $p['custom_fields'] ?? array() as $veld ) {
			if ( 'Fase' === ( $veld['name'] ?? '' ) && ! empty( $veld['value'] ) ) {
				$fase[ (int) $p['id'] ] = (string) $veld['value'];
			}
		}
	}
	$publiek = rtrim( (string) ( getenv( 'DG_REDMINE_URL' ) ?: '' ), '/' );
	$uit     = array();
	foreach ( $nieuws['news'] ?? array() as $n ) {
		$uit[] = array(
			'project'  => (string) ( $n['project']['name'] ?? '' ),
			'titel'    => (string) ( $n['title'] ?? '' ),
			'tekst'    => (string) ( $n['summary'] ?: wp_trim_words( (string) ( $n['description'] ?? '' ), 18 ) ),
			'datum'    => strtotime( (string) ( $n['created_on'] ?? '' ) ) ?: 0,
			'fase'     => $fase[ (int) ( $n['project']['id'] ?? 0 ) ] ?? '',
			'reacties' => isset( $n['comments_count'] ) ? (int) $n['comments_count'] : null,
			'url'      => $publiek ? $publiek . '/news/' . (int) $n['id'] : '',
		);
	}
	set_transient( 'dg_db_projectupdates', $uit, DG_DASHBOARD_CACHE );
	return $uit;
}

function dg_db_projectupdates(): string {
	$updates = dg_db_projectupdates_gegevens();
	$publiek = rtrim( (string) ( getenv( 'DG_REDMINE_URL' ) ?: '' ), '/' );
	if ( ! $updates ) {
		$inhoud = '<p class="dg-db-leeg">Nog geen projectupdates. Een update plaats je als <em>nieuws</em> bij een project in Redmine.</p>';
	} else {
		$inhoud = '<ul class="dg-db-lijst">';
		foreach ( $updates as $u ) {
			$fase    = $u['fase'] ? ' <span class="dg-db-fase dg-db-fase-' . esc_attr( sanitize_title( $u['fase'] ) ) . '">' . esc_html( $u['fase'] ) . '</span>' : '';
			$inhoud .= '<li class="dg-db-update"><span class="dg-db-tegel"><span class="dg-icoon dg-icoon-lagen" aria-hidden="true"></span></span><div>'
				. '<a class="dg-db-titel" href="' . esc_url( $u['url'] ) . '">' . esc_html( $u['project'] ) . '</a>' . $fase
				. '<p><strong>' . esc_html( $u['titel'] ) . '.</strong> ' . esc_html( $u['tekst'] ) . '</p>'
				. '<small><span class="dg-icoon dg-icoon-kalender" aria-hidden="true"></span>' . esc_html( wp_date( 'j M Y', $u['datum'] ) )
				. ( null !== $u['reacties'] ? ' <span class="dg-icoon dg-icoon-forum" aria-hidden="true"></span>' . (int) $u['reacties'] . ' reacties' : '' ) . '</small></div></li>';
		}
		$inhoud .= '</ul>';
	}
	return dg_db_blok( 'dg-db-updates', 'lagen', 'Projectupdates', 'Alles bekijken', $publiek ? $publiek . '/news' : '', $inhoud );
}

/** De vier onderwerpen in de fora waar het laatst iets gebeurde. */
function dg_db_discussies(): string {
	$inhoud = '<p class="dg-db-leeg">Nog geen discussies. Begin er een!</p>';
	if ( function_exists( 'bbp_get_topic_post_type' ) ) {
		$q = new WP_Query(
			array(
				'post_type'      => bbp_get_topic_post_type(),
				'post_status'    => array( 'publish', 'closed' ),
				'posts_per_page' => 4,
				'meta_key'       => '_bbp_last_active_time', // phpcs:ignore WordPress.DB.SlowDBQuery
				'orderby'        => 'meta_value',
				'order'          => 'DESC',
				'no_found_rows'  => true,
			)
		);
		if ( $q->have_posts() ) {
			$inhoud = '<ul class="dg-db-lijst">';
			foreach ( $q->posts as $t ) {
				$laatst  = strtotime( (string) get_post_meta( $t->ID, '_bbp_last_active_time', true ) . ' UTC' ) ?: strtotime( $t->post_date_gmt . ' UTC' );
				$inhoud .= '<li class="dg-db-discussie"><span class="dg-icoon dg-icoon-forum" aria-hidden="true"></span><div><a class="dg-db-titel" href="' . esc_url( get_permalink( $t ) ) . '">' . esc_html( get_the_title( $t ) ) . '</a>'
					. '<small>' . esc_html( bbp_get_topic_forum_title( $t->ID ) ) . '</small></div>'
					. '<span class="dg-db-meta"><span class="dg-icoon dg-icoon-forum" aria-hidden="true"></span>' . (int) bbp_get_topic_reply_count( $t->ID, true ) . '</span>'
					. '<span class="dg-db-meta dg-db-tijd">' . esc_html( dg_db_wanneer( (int) $laatst ) ) . '</span></li>';
			}
			$inhoud .= '</ul>';
		}
	}
	return dg_db_blok( 'dg-db-discussies', 'forum', 'Recente discussies', 'Naar de fora', dg_db_forum_url(), $inhoud );
}

// ---------------------------------------------------------------------------
// Rechts: samenwerkingen, actieve leden, evenementen
// ---------------------------------------------------------------------------

/** Uit inhoud/paginas/samenwerkingen.md (kopje "dashboard: samenwerkingen"), via wp dg inhoud. */
function dg_db_samenwerkingen(): string {
	$lijst = (array) get_option( 'dg_dashboard_samenwerkingen', array() );
	if ( ! $lijst ) {
		return '';
	}
	$inhoud = '<ul class="dg-db-lijst">';
	foreach ( array_slice( $lijst, 0, 5 ) as $s ) {
		$status  = (string) ( $s['label'] ?? '' );
		$inhoud .= '<li class="dg-db-samen"><span class="dg-icoon dg-icoon-' . esc_attr( $s['icoon'] ?: 'hand' ) . '" aria-hidden="true"></span><div><strong>' . esc_html( $s['naam'] ) . '</strong><small>' . esc_html( $s['tekst'] ) . '</small></div>'
			. ( $status ? '<span class="dg-db-status dg-db-status-' . esc_attr( sanitize_title( $status ) ) . '">' . esc_html( $status ) . '</span>' : '' ) . '</li>';
	}
	$inhoud .= '</ul>';
	return dg_db_blok( 'dg-db-samenwerkingen', 'hand', 'Samenwerkingen', 'Bekijk alles', home_url( '/samenwerkingen/' ), $inhoud );
}

function dg_db_leden(): string {
	if ( ! function_exists( 'bp_has_members' ) ) {
		return '';
	}
	$totaal = (int) bp_core_get_total_member_count();
	$query  = new BP_User_Query( array( 'type' => 'active', 'per_page' => 6, 'populate_extras' => false ) );
	$fotos  = '';
	foreach ( $query->results as $lid ) {
		$fotos .= '<a href="' . esc_url( bp_members_get_user_url( $lid->ID ) ) . '" title="' . esc_attr( $lid->display_name ) . '">'
			. bp_core_fetch_avatar( array( 'item_id' => $lid->ID, 'type' => 'thumb', 'width' => 46, 'height' => 46, 'html' => true, 'alt' => $lid->display_name ) ) . '</a>';
	}
	$rest   = $totaal - count( $query->results );
	$fotos .= $rest > 0 ? '<span class="dg-db-meer">+' . number_format_i18n( $rest ) . '</span>' : '';
	$titel  = 'Actieve leden <span class="dg-db-aantal">(' . number_format_i18n( $totaal ) . ')</span>';
	return dg_db_blok( 'dg-db-leden', 'mensen', $titel, 'Alles bekijken', bp_get_members_directory_permalink(), '<div class="dg-db-fotos">' . $fotos . '</div>' );
}

function dg_db_evenementen_url(): string {
	if ( function_exists( 'civicrm_initialize' ) && civicrm_initialize() ) {
		try {
			return (string) CRM_Utils_System::url( 'civicrm/event/ical', 'reset=1&list=1&html=1', true, null, false, true );
		} catch ( Throwable $e ) {
			unset( $e );
		}
	}
	return home_url( '/' );
}

/** De volgende twee openbare evenementen uit CiviCRM, met het aantal aanmeldingen. */
function dg_db_evenementen(): string {
	$items = array();
	if ( function_exists( 'civicrm_initialize' ) && civicrm_initialize() && class_exists( '\Civi\Api4\Event' ) ) {
		try {
			$events = \Civi\Api4\Event::get( false )
				->addSelect( 'id', 'title', 'start_date', 'loc_block_id.address_id.city' )
				->addWhere( 'is_public', '=', true )
				->addWhere( 'is_active', '=', true )
				->addWhere( 'start_date', '>=', 'now' )
				->addOrderBy( 'start_date', 'ASC' )
				->setLimit( 2 )
				->execute();
			foreach ( $events as $e ) {
				$aantal  = \Civi\Api4\Participant::get( false )
					->selectRowCount()
					->addWhere( 'event_id', '=', $e['id'] )
					->addWhere( 'status_id:name', 'IN', array( 'Registered', 'Attended' ) )
					->execute()
					->count();
				$items[] = array(
					'titel'  => (string) $e['title'],
					'tijd'   => strtotime( (string) $e['start_date'] ),
					'plaats' => (string) ( $e['loc_block_id.address_id.city'] ?? '' ),
					'aantal' => (int) $aantal,
					'url'    => (string) CRM_Utils_System::url( 'civicrm/event/info', 'reset=1&id=' . (int) $e['id'], true, null, false, true ),
				);
			}
		} catch ( Throwable $fout ) {
			$items = array();
		}
	}
	if ( ! $items ) {
		$inhoud = '<p class="dg-db-leeg">Nog geen evenementen gepland. Evenementen maak je in CiviCRM (Evenementen → Nieuw evenement, openbaar).</p>';
	} else {
		$inhoud = '<ul class="dg-db-lijst">';
		foreach ( $items as $i ) {
			$inhoud .= '<li class="dg-db-evenement"><span class="dg-db-datum"><b>' . esc_html( wp_date( 'j', $i['tijd'] ) ) . '</b>' . esc_html( wp_date( 'M', $i['tijd'] ) ) . '</span><div>'
				. '<a class="dg-db-titel" href="' . esc_url( $i['url'] ) . '">' . esc_html( $i['titel'] ) . '</a><small>'
				. '<span class="dg-icoon dg-icoon-kompas" aria-hidden="true"></span>' . esc_html( $i['plaats'] ?: 'Online' )
				. ' <span class="dg-icoon dg-icoon-mensen" aria-hidden="true"></span>' . (int) $i['aantal'] . ' deelnemers</small></div></li>';
		}
		$inhoud .= '</ul>';
	}
	return dg_db_blok( 'dg-db-evenementen', 'kalender', 'Aankomende evenementen', 'Kalender', dg_db_evenementen_url(), $inhoud );
}

/** De kennisbank (Wiki.js) op kennis.<domein>. */
function dg_db_kennisbank(): string {
	return 'https://kennis.' . wp_parse_url( home_url(), PHP_URL_HOST ) . '/';
}
